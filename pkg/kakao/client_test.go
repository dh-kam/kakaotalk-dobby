package kakao

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ModularServices(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/v2/user/me":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 12345, "kakao_account": {"profile": {"nickname": "Alice"}}}`))
		case "/v2/api/talk/memo/default/send":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result_code": 0}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	store := NewMemoryTokenStore(&TokenInfo{
		AccessToken: "valid-tok",
		ExpiresIn:   3600,
		CreatedAt:   time.Now(),
	})

	client := NewClient(ClientConfig{
		APIBaseURL: ts.URL,
		TokenStore: store,
	})

	// User service
	user, err := client.User().GetMe(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(12345), user.ID)
	assert.Equal(t, "Alice", user.KakaoAccount.Profile.Nickname)

	// Memo service
	err = client.Memo().SendText(context.Background(), TextMessageRequest{
		Text: "Hello from modular client",
	})
	require.NoError(t, err)
}

func TestClient_AutoRefreshToken(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oauth/token", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"access_token": "refreshed-tok-999",
			"token_type": "bearer",
			"expires_in": 3600
		}`))
	}))
	defer authServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer refreshed-tok-999", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result_code": 0}`))
	}))
	defer apiServer.Close()

	store := NewMemoryTokenStore(&TokenInfo{
		AccessToken:  "expired-token",
		RefreshToken: "valid-refresh-token",
		ExpiresIn:    10,
		CreatedAt:    time.Now().Add(-100 * time.Second),
	})

	client := NewClient(ClientConfig{
		AuthBaseURL: authServer.URL,
		APIBaseURL:  apiServer.URL,
		ClientID:    "client-id",
		TokenStore:  store,
	})

	err := client.Memo().SendText(context.Background(), TextMessageRequest{Text: "Test"})
	require.NoError(t, err)

	updatedToken, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "refreshed-tok-999", updatedToken.AccessToken)
}
