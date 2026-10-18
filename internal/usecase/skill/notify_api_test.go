package skill

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/internal/usecase/notify"
	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupNotifyTestKakaoServer(t *testing.T) *httptest.Server {
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
					{"profile_nickname": "아빠", "uuid": "uuid-dad", "id": 111, "allowed_msg": true},
					{"profile_nickname": "엄마", "uuid": "uuid-mom", "id": 222, "allowed_msg": true}
				],
				"total_count": 2
			}`))
		case "/v1/api/talk/friends/message/default/send", "/v1/api/talk/friends/message/send":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"successful_receiver_uuids": ["user-1"], "failure_info": []}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func createNotifyTestRegistry(apiURL string) kakao.ClientRegistry {
	reg := kakao.NewClientRegistry()

	dobbyClient := kakao.NewClient(kakao.ClientConfig{
		APIBaseURL: apiURL,
		ClientID:   "dobby-client",
		TokenStore: kakao.NewMemoryTokenStore(&kakao.TokenInfo{
			AccessToken: "valid-tok",
			ExpiresIn:   3600,
			CreatedAt:   time.Now(),
		}),
	})
	infraClient := kakao.NewClient(kakao.ClientConfig{
		APIBaseURL: apiURL,
		ClientID:   "infra-client",
		TokenStore: kakao.NewMemoryTokenStore(&kakao.TokenInfo{
			AccessToken: "valid-tok",
			ExpiresIn:   3600,
			CreatedAt:   time.Now(),
		}),
	})

	reg.Register("dobby", dobbyClient, true)
	reg.Register("infra", infraClient, false)
	return reg
}

func TestNotifyAPI_Authentication(t *testing.T) {
	ts := setupNotifyTestKakaoServer(t)
	defer ts.Close()

	reg := createNotifyTestRegistry(ts.URL)
	handler := newNotifyHandler(reg, "secret-token", "0xc0de1ab")

	t.Run("missing token returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/message/send", strings.NewReader(`{"text":"test"}`))
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("wrong token returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/message/send", strings.NewReader(`{"text":"test"}`))
		req.Header.Set("Authorization", "Bearer wrong-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("valid Bearer token succeeds", func(t *testing.T) {
		payload := `{"text":"Hello from cluster alert"}`
		req := httptest.NewRequest(http.MethodPost, "/api/message/send", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer secret-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp notify.SendMessageResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "dobby", resp.Bot)
	})

	t.Run("valid X-Admin-Token succeeds", func(t *testing.T) {
		payload := `{"bot":"infra", "text":"Alert: K8s node disk full"}`
		req := httptest.NewRequest(http.MethodPost, "/api/message/send", strings.NewReader(payload))
		req.Header.Set("X-Admin-Token", "secret-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp notify.SendMessageResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "infra", resp.Bot)
	})
}

func TestNotifyAPI_ValidationAndRouting(t *testing.T) {
	ts := setupNotifyTestKakaoServer(t)
	defer ts.Close()

	reg := createNotifyTestRegistry(ts.URL)
	handler := newNotifyHandler(reg, "secret-token", "0xc0de1ab")

	t.Run("unknown bot returns 404", func(t *testing.T) {
		payload := `{"bot":"unknown-bot", "text":"test"}`
		req := httptest.NewRequest(http.MethodPost, "/api/message/send", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer secret-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("empty text returns 400", func(t *testing.T) {
		payload := `{"bot":"infra"}`
		req := httptest.NewRequest(http.MethodPost, "/api/message/send", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer secret-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unsupported target returns 400", func(t *testing.T) {
		payload := `{"target":"sms", "text":"hello"}`
		req := httptest.NewRequest(http.MethodPost, "/api/message/send", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer secret-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("friends target with receivers succeeds", func(t *testing.T) {
		payload := `{"target":"friends", "text":"team alert", "receiver_uuids":["user-1"]}`
		req := httptest.NewRequest(http.MethodPost, "/api/message/send", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer secret-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp notify.SendMessageResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, notify.TargetFriends, resp.Target)
		assert.Equal(t, 1, resp.DeliveredCount)
	})

	t.Run("channel target succeeds", func(t *testing.T) {
		payload := `{"target":"channel", "text":"channel alert", "channel_id":"0xc0de1ab"}`
		req := httptest.NewRequest(http.MethodPost, "/api/message/send", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer secret-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp notify.SendMessageResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, notify.TargetChannel, resp.Target)
	})

	t.Run("payload exceeding 64KB is rejected", func(t *testing.T) {
		hugeText := strings.Repeat("A", 70*1024)
		payload := map[string]string{"text": hugeText}
		data, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/message/send", bytes.NewReader(data))
		req.Header.Set("Authorization", "Bearer secret-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestBotsListAPI(t *testing.T) {
	ts := setupNotifyTestKakaoServer(t)
	defer ts.Close()

	reg := createNotifyTestRegistry(ts.URL)
	handler := newBotsListHandler(reg, "secret-token")

	t.Run("unauthorized access returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/message/bots", nil)
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("authorized access returns bots list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/message/bots", nil)
		req.Header.Set("Authorization", "Bearer secret-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var result struct {
			Bots  []kakao.BotInfo `json:"bots"`
			Count int             `json:"count"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, 2, result.Count)
		require.Len(t, result.Bots, 2)
		assert.Equal(t, "dobby", result.Bots[0].Name)
		assert.True(t, result.Bots[0].IsDefault)
		assert.Equal(t, "infra", result.Bots[1].Name)
		assert.False(t, result.Bots[1].IsDefault)
	})
}

func TestFriendsListAPI(t *testing.T) {
	ts := setupNotifyTestKakaoServer(t)
	defer ts.Close()

	reg := createNotifyTestRegistry(ts.URL)
	handler := newFriendsListHandler(reg, "secret-token")

	t.Run("unauthorized access returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/friends", nil)
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("authorized access returns friends list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/friends?bot=dobby", nil)
		req.Header.Set("Authorization", "Bearer secret-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var result kakao.FriendsResponse
		err := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, 2, result.TotalCount)
		require.Len(t, result.Elements, 2)
		assert.Equal(t, "아빠", result.Elements[0].ProfileNickname)
		assert.Equal(t, "uuid-dad", result.Elements[0].UUID)
		assert.True(t, result.Elements[0].AllowedMsg)
	})
}
