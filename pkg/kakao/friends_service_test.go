package kakao

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockGuard struct {
	validateFn func(ctx context.Context, receiverUUIDs []string, messageSummary string) ([]string, error)
}

func (m *mockGuard) ValidateSend(ctx context.Context, receiverUUIDs []string, messageSummary string) ([]string, error) {
	if m.validateFn != nil {
		return m.validateFn(ctx, receiverUUIDs, messageSummary)
	}
	return receiverUUIDs, nil
}

func TestFriendsService_GetFriendsAndSend(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/v1/api/talk/friends" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"elements": [{"id": 1, "uuid": "uuid-1", "profile_nickname": "Friend1", "allowed_msg": true}],
				"total_count": 1
			}`))
			return
		}

		if r.URL.Path == "/v1/api/talk/friends/message/default/send" {
			assert.NoError(t, r.ParseForm())
			assert.Contains(t, r.Form.Get("receiver_uuids"), "uuid-1")
			assert.Contains(t, r.Form.Get("template_object"), "Hello Friend")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"successful_receiver_uuids": ["uuid-1"]}`))
			return
		}

		http.NotFound(w, r)
	}))
	defer ts.Close()

	svc := NewFriendsService(ts.URL, nil, func(ctx context.Context) (string, error) {
		return "tok", nil
	})

	friends, err := svc.GetFriends(context.Background(), FriendsQueryOptions{Limit: 5})
	require.NoError(t, err)
	assert.Equal(t, 1, friends.TotalCount)
	assert.Equal(t, "Friend1", friends.Elements[0].ProfileNickname)

	res, err := svc.SendText(context.Background(), []string{"uuid-1"}, TextMessageRequest{
		Text: "Hello Friend",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"uuid-1"}, res.SuccessfulReceiverUUIDs)
}

func TestFriendsService_WithFriendGuard_Blocked(t *testing.T) {
	var apiCalled bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"successful_receiver_uuids": ["uuid-unverified"]}`))
	}))
	defer ts.Close()

	guardErr := errors.New("guard: recipient unverified")
	guard := &mockGuard{
		validateFn: func(ctx context.Context, receiverUUIDs []string, messageSummary string) ([]string, error) {
			assert.Contains(t, messageSummary, "Blocked Message")
			return nil, guardErr
		},
	}

	svc := NewFriendsService(ts.URL, nil, func(ctx context.Context) (string, error) {
		return "tok", nil
	}, guard)

	res, err := svc.SendText(context.Background(), []string{"uuid-unverified"}, TextMessageRequest{
		Text: "Blocked Message",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, guardErr))
	assert.Nil(t, res)
	assert.False(t, apiCalled, "Kakao API must NOT be called when guard returns error")
}

func TestFriendsService_WithFriendGuard_Filtered(t *testing.T) {
	var sentUUIDs string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		sentUUIDs = r.Form.Get("receiver_uuids")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"successful_receiver_uuids": ["uuid-allowed"]}`))
	}))
	defer ts.Close()

	guard := &mockGuard{
		validateFn: func(ctx context.Context, receiverUUIDs []string, messageSummary string) ([]string, error) {
			// Filters out "uuid-blocked", keeps "uuid-allowed"
			return []string{"uuid-allowed"}, nil
		},
	}

	svc := NewFriendsService(ts.URL, nil, func(ctx context.Context) (string, error) {
		return "tok", nil
	}, guard)

	res, err := svc.SendText(context.Background(), []string{"uuid-allowed", "uuid-blocked"}, TextMessageRequest{
		Text: "Partially filtered",
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"uuid-allowed"}, res.SuccessfulReceiverUUIDs)
	assert.Contains(t, sentUUIDs, "uuid-allowed")
	assert.NotContains(t, sentUUIDs, "uuid-blocked")
}

func TestClient_FriendGuard_DisabledBypassesDefault(t *testing.T) {
	guardCalled := false
	guard := &mockGuard{
		validateFn: func(ctx context.Context, receiverUUIDs []string, messageSummary string) ([]string, error) {
			guardCalled = true
			return nil, errors.New("should not be called")
		},
	}

	SetDefaultFriendGuard(guard)
	defer SetDefaultFriendGuard(nil)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"successful_receiver_uuids": ["uuid-admin"]}`))
	}))
	defer ts.Close()

	// Client with DisableFriendGuard: true
	client := NewClient(ClientConfig{
		APIBaseURL:         ts.URL,
		DisableFriendGuard: true,
	})

	res, err := client.Friends().SendText(context.Background(), []string{"uuid-admin"}, TextMessageRequest{
		Text: "Admin Alert",
	})

	require.NoError(t, err)
	assert.False(t, guardCalled, "Default friend guard must be bypassed when DisableFriendGuard is true")
	assert.Equal(t, []string{"uuid-admin"}, res.SuccessfulReceiverUUIDs)
}
