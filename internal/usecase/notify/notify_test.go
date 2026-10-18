package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestKakaoServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/api/talk/memo/default/send", "/v2/api/talk/memo/send":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result_code": 0}`))
		case "/v1/api/talk/friends":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"elements": [
					{"profile_nickname": "아빠", "uuid": "uuid-1", "id": 111, "allowed_msg": true},
					{"profile_nickname": "엄마", "uuid": "uuid-2", "id": 222, "allowed_msg": true}
				],
				"total_count": 2
			}`))
		case "/v1/api/talk/friends/message/default/send", "/v1/api/talk/friends/message/send":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"successful_receiver_uuids": ["uuid-1", "uuid-2"],
				"failure_info": []
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func createTestClient(apiURL, clientID string) kakao.Client {
	tokenStore := kakao.NewMemoryTokenStore(&kakao.TokenInfo{
		AccessToken:  "mock-access-token",
		RefreshToken: "mock-refresh-token",
		ExpiresIn:    3600,
		CreatedAt:    time.Now(),
	})
	return kakao.NewClient(kakao.ClientConfig{
		APIBaseURL: apiURL,
		ClientID:   clientID,
		TokenStore: tokenStore,
	})
}

func TestNormalizeTarget(t *testing.T) {
	cases := []struct {
		input    TargetType
		expected TargetType
		valid    bool
	}{
		{"", TargetMemo, true},
		{"memo", TargetMemo, true},
		{"me", TargetMemo, true},
		{"self", TargetMemo, true},
		{"friends", TargetFriends, true},
		{"friend", TargetFriends, true},
		{"channel", TargetChannel, true},
		{"invalid", "invalid", false},
	}

	for _, tc := range cases {
		norm, ok := NormalizeTarget(tc.input)
		assert.Equal(t, tc.valid, ok)
		assert.Equal(t, tc.expected, norm)
	}
}

func TestNotifyUseCase_ValidationErrors(t *testing.T) {
	reg := kakao.NewClientRegistry()
	uc := NewNotifyUseCase(reg, nil)
	ctx := context.Background()

	// Unsupported target
	_, err := uc.Send(ctx, SendMessageRequest{
		Target: "unknown",
		Text:   "hello",
	})
	require.ErrorIs(t, err, ErrInvalidRequest)
	assert.Contains(t, err.Error(), "unsupported target")

	// Friends target without receiver UUIDs
	_, err = uc.Send(ctx, SendMessageRequest{
		Target: TargetFriends,
		Text:   "hello",
	})
	require.ErrorIs(t, err, ErrInvalidRequest)
	assert.Contains(t, err.Error(), "receiver_uuids is required")

	// Empty content
	_, err = uc.Send(ctx, SendMessageRequest{
		Target: TargetMemo,
	})
	require.ErrorIs(t, err, ErrInvalidRequest)
	assert.Contains(t, err.Error(), "text, title, or template_id is required")
}

func TestNotifyUseCase_MultiBotMemoSend(t *testing.T) {
	ts := setupTestKakaoServer(t)
	defer ts.Close()

	dobbyClient := createTestClient(ts.URL, "dobby-id")
	infraClient := createTestClient(ts.URL, "infra-id")

	reg := kakao.NewClientRegistry()
	reg.Register("dobby", dobbyClient, true)
	reg.Register("infra", infraClient, false)

	uc := NewNotifyUseCase(reg, nil)
	ctx := context.Background()

	t.Run("send text via default bot (dobby)", func(t *testing.T) {
		res, err := uc.Send(ctx, SendMessageRequest{
			Target: TargetMemo,
			Text:   "Hello from Dobby default",
		})
		require.NoError(t, err)
		assert.True(t, res.Success)
		assert.Equal(t, "dobby", res.Bot)
		assert.Equal(t, TargetMemo, res.Target)
		assert.Equal(t, MessageTypeText, res.Type)
		assert.Equal(t, 1, res.DeliveredCount)
	})

	t.Run("send feed via explicit bot (infra)", func(t *testing.T) {
		res, err := uc.Send(ctx, SendMessageRequest{
			Bot:         "infra",
			Target:      TargetMemo,
			Title:       "Kubernetes Pod Crash Alert",
			Description: "Deployment api-gateway crashed 5 times",
			WebURL:      "https://grafana.0xc0de1ab.dev",
			ButtonTitle: "View in Grafana",
		})
		require.NoError(t, err)
		assert.True(t, res.Success)
		assert.Equal(t, "infra", res.Bot)
		assert.Equal(t, TargetMemo, res.Target)
		assert.Equal(t, MessageTypeFeed, res.Type)
		assert.Equal(t, 1, res.DeliveredCount)
	})

	t.Run("send custom template", func(t *testing.T) {
		res, err := uc.Send(ctx, SendMessageRequest{
			Bot:          "infra",
			Target:       TargetMemo,
			TemplateID:   998877,
			TemplateArgs: map[string]string{"title": "test", "body": "arg"},
		})
		require.NoError(t, err)
		assert.True(t, res.Success)
		assert.Equal(t, "infra", res.Bot)
		assert.Equal(t, MessageTypeCustom, res.Type)
	})

	t.Run("unknown bot returns error", func(t *testing.T) {
		_, err := uc.Send(ctx, SendMessageRequest{
			Bot:    "nonexistent",
			Target: TargetMemo,
			Text:   "will fail",
		})
		require.Error(t, err)
		require.ErrorIs(t, err, kakao.ErrBotNotFound)
	})
}

func TestNotifyUseCase_FriendsSend(t *testing.T) {
	ts := setupTestKakaoServer(t)
	defer ts.Close()

	client := createTestClient(ts.URL, "infra-id")
	reg := kakao.NewClientRegistry()
	reg.Register("infra", client, true)

	uc := NewNotifyUseCase(reg, nil)
	ctx := context.Background()

	res, err := uc.Send(ctx, SendMessageRequest{
		Bot:           "infra",
		Target:        TargetFriends,
		Text:          "Alert to team",
		ReceiverUUIDs: []string{"uuid-1", "uuid-2"},
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, 2, res.DeliveredCount)
	assert.Equal(t, 0, res.FailedCount)
	require.NotNil(t, res.Details)
	assert.Equal(t, []string{"uuid-1", "uuid-2"}, res.Details.SuccessfulReceivers)
}

func TestNotifyUseCase_FamilySend(t *testing.T) {
	ts := setupTestKakaoServer(t)
	defer ts.Close()

	client := createTestClient(ts.URL, "dobby")
	reg := kakao.NewClientRegistry()
	reg.Register("dobby", client, true)

	uc := NewNotifyUseCase(reg, nil)
	ctx := context.Background()

	t.Run("family broadcast with auto resolved friends", func(t *testing.T) {
		res, err := uc.Send(ctx, SendMessageRequest{
			Bot:    "dobby",
			Target: TargetFamily,
			Text:   "Family announcement",
		})
		require.NoError(t, err)
		assert.True(t, res.Success)
		assert.Equal(t, TargetFamily, res.Target)
		assert.Equal(t, 3, res.DeliveredCount) // 1 (self) + 2 (friends)
		assert.Equal(t, 0, res.FailedCount)
		require.NotNil(t, res.Details)
		assert.Equal(t, []string{"me", "uuid-1", "uuid-2"}, res.Details.SuccessfulReceivers)
	})
}

func TestNotifyUseCase_ChannelSend(t *testing.T) {
	ts := setupTestKakaoServer(t)
	defer ts.Close()

	client := createTestClient(ts.URL, "channel-bot")
	reg := kakao.NewClientRegistry()
	reg.Register("channel-bot", client, true)

	channelSender := NewDefaultChannelSender("0xc0de1ab")
	uc := NewNotifyUseCase(reg, channelSender)
	ctx := context.Background()

	res, err := uc.Send(ctx, SendMessageRequest{
		Target:    TargetChannel,
		Text:      "Weekly Newsletter Published",
		ChannelID: "0xc0de1ab",
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, TargetChannel, res.Target)
	assert.Equal(t, 1, res.DeliveredCount)
	require.NotNil(t, res.Details)
	assert.Equal(t, "0xc0de1ab", res.Details.ChannelInfo["channel_id"])
}

func TestSendMessageRequest_JSONSerialization(t *testing.T) {
	rawJSON := `{
		"bot": "infra",
		"target": "friends",
		"text": "Server down",
		"receiver_uuids": ["user1", "user2"],
		"template_args": {"cluster": "prod"}
	}`

	var req SendMessageRequest
	err := json.Unmarshal([]byte(rawJSON), &req)
	require.NoError(t, err)
	assert.Equal(t, "infra", req.Bot)
	assert.Equal(t, TargetFriends, req.Target)
	assert.Equal(t, "Server down", req.Text)
	assert.Equal(t, []string{"user1", "user2"}, req.ReceiverUUIDs)
	assert.Equal(t, "prod", req.TemplateArgs["cluster"])
}
