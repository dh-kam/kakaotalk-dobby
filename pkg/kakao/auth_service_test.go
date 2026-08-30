package kakao

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthService_GetAuthURL(t *testing.T) {
	svc := NewAuthService("https://kauth.kakao.com", "https://kapi.kakao.com", "my-client-id", "my-secret", "http://localhost:8080/callback", nil, nil)
	url := svc.GetAuthURL([]string{ScopeTalkMessage, ScopeFriends})

	assert.Contains(t, url, "client_id=my-client-id")
	assert.Contains(t, url, "redirect_uri=http%3A%2F%2Flocalhost%3A8080%2Fcallback")
	assert.Contains(t, url, "scope=talk_message%2Cfriends")
}

func TestAuthService_ExchangeCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oauth/token", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.NoError(t, r.ParseForm())
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "test-code", r.Form.Get("code"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"access_token": "acc-123",
			"token_type": "bearer",
			"refresh_token": "ref-456",
			"expires_in": 3600
		}`))
	}))
	defer ts.Close()

	svc := NewAuthService(ts.URL, ts.URL, "client", "secret", "uri", nil, nil)
	token, err := svc.ExchangeCode(context.Background(), "test-code")
	require.NoError(t, err)
	assert.Equal(t, "acc-123", token.AccessToken)
	assert.Equal(t, "ref-456", token.RefreshToken)
	assert.False(t, token.IsExpired())
}

func TestAuthService_LogoutAndUnlink(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-tok", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": 99999}`))
	}))
	defer ts.Close()

	tokenProvider := func(ctx context.Context) (string, error) {
		return "test-tok", nil
	}

	svc := NewAuthService(ts.URL, ts.URL, "client", "secret", "uri", nil, tokenProvider)

	logoutID, err := svc.Logout(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(99999), logoutID)

	unlinkID, err := svc.Unlink(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(99999), unlinkID)
}
