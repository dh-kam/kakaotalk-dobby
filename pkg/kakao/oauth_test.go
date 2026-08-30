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

func TestOAuthClient_GetAuthCodeURL(t *testing.T) {
	client := NewOAuthClient(OAuthConfig{
		ClientID:    "test-client-id",
		RedirectURI: "http://localhost:8080/callback",
	})

	url := client.GetAuthCodeURL([]string{ScopeTalkMessage, ScopeFriends})
	assert.Contains(t, url, "client_id=test-client-id")
	assert.Contains(t, url, "redirect_uri=http%3A%2F%2Flocalhost%3A8080%2Fcallback")
	assert.Contains(t, url, "scope=talk_message%2Cfriends")
	assert.Contains(t, url, "response_type=code")
}

func TestOAuthClient_ExchangeToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oauth/token", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.NoError(t, r.ParseForm())
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "test-client-id", r.Form.Get("client_id"))
		assert.Equal(t, "test-code", r.Form.Get("code"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"access_token": "access-token-123",
			"token_type": "bearer",
			"refresh_token": "refresh-token-456",
			"expires_in": 21600,
			"scope": "talk_message"
		}`))
	}))
	defer ts.Close()

	client := NewOAuthClient(OAuthConfig{
		AuthBaseURL: ts.URL,
		ClientID:    "test-client-id",
		RedirectURI: "http://localhost:8080/callback",
	})

	token, err := client.ExchangeToken(context.Background(), "test-code")
	require.NoError(t, err)
	assert.Equal(t, "access-token-123", token.AccessToken)
	assert.Equal(t, "refresh-token-456", token.RefreshToken)
	assert.False(t, token.IsExpired())
}

func TestOAuthClient_RefreshToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oauth/token", r.URL.Path)
		assert.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "refresh-token-456", r.Form.Get("refresh_token"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"access_token": "new-access-token-789",
			"token_type": "bearer",
			"expires_in": 21600
		}`))
	}))
	defer ts.Close()

	client := NewOAuthClient(OAuthConfig{
		AuthBaseURL: ts.URL,
		ClientID:    "test-client-id",
	})

	refreshed, err := client.RefreshToken(context.Background(), "refresh-token-456")
	require.NoError(t, err)
	assert.Equal(t, "new-access-token-789", refreshed.AccessToken)
	assert.Equal(t, "refresh-token-456", refreshed.RefreshToken)
}

func TestTokenInfo_Expiry(t *testing.T) {
	token := &TokenInfo{
		AccessToken: "test",
		ExpiresIn:   100,
		CreatedAt:   time.Now().Add(-50 * time.Second),
	}
	assert.True(t, token.IsExpired()) // 100 - 60 = 40s expiry window

	freshToken := &TokenInfo{
		AccessToken: "test",
		ExpiresIn:   3600,
		CreatedAt:   time.Now(),
	}
	assert.False(t, freshToken.IsExpired())
}
