package kakao

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
