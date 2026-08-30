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

func TestClient_SendMeText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/api/talk/memo/default/send", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer test-access-token", r.Header.Get("Authorization"))

		assert.NoError(t, r.ParseForm())
		tmpl := r.Form.Get("template_object")
		assert.Contains(t, tmpl, `"object_type":"text"`)
		assert.Contains(t, tmpl, `"text":"Hello Kakao"`)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result_code": 0}`))
	}))
	defer ts.Close()

	store := NewMemoryTokenStore(&TokenInfo{
		AccessToken: "test-access-token",
		ExpiresIn:   3600,
		CreatedAt:   time.Now(),
	})

	client := NewClient(ClientConfig{
		APIBaseURL: ts.URL,
		TokenStore: store,
	})

	err := client.SendMeText(context.Background(), "Hello Kakao", "https://example.com", "Open")
	assert.NoError(t, err)
}

func TestClient_SendFriendsText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/api/talk/friends/message/default/send", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer test-access-token", r.Header.Get("Authorization"))

		assert.NoError(t, r.ParseForm())
		assert.Contains(t, r.Form.Get("receiver_uuids"), "uuid-1")
		assert.Contains(t, r.Form.Get("template_object"), "Hello Friend")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"successful_receiver_uuids": ["uuid-1"]
		}`))
	}))
	defer ts.Close()

	store := NewMemoryTokenStore(&TokenInfo{
		AccessToken: "test-access-token",
		ExpiresIn:   3600,
		CreatedAt:   time.Now(),
	})

	client := NewClient(ClientConfig{
		APIBaseURL: ts.URL,
		TokenStore: store,
	})

	res, err := client.SendFriendsText(context.Background(), []string{"uuid-1"}, "Hello Friend", "https://example.com", "View")
	require.NoError(t, err)
	assert.Equal(t, []string{"uuid-1"}, res.SuccessfulReceiverUUIDs)
}

func TestClient_GetMe(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/user/me", r.URL.Path)
		assert.Equal(t, "Bearer test-access-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": 12345678,
			"kakao_account": {
				"profile": {
					"nickname": "TestUser"
				}
			}
		}`))
	}))
	defer ts.Close()

	store := NewMemoryTokenStore(&TokenInfo{
		AccessToken: "test-access-token",
		ExpiresIn:   3600,
		CreatedAt:   time.Now(),
	})

	client := NewClient(ClientConfig{
		APIBaseURL: ts.URL,
		TokenStore: store,
	})

	profile, err := client.GetMe(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(12345678), profile.ID)
	assert.Equal(t, "TestUser", profile.KakaoAccount.Profile.Nickname)
}

func TestClient_AutoRefreshToken(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oauth/token", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"access_token": "refreshed-token-999",
			"token_type": "bearer",
			"expires_in": 3600
		}`))
	}))
	defer authServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer refreshed-token-999", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result_code": 0}`))
	}))
	defer apiServer.Close()

	oauthClient := NewOAuthClient(OAuthConfig{
		AuthBaseURL: authServer.URL,
		ClientID:    "test-client",
	})

	store := NewMemoryTokenStore(&TokenInfo{
		AccessToken:  "expired-token",
		RefreshToken: "valid-refresh-token",
		ExpiresIn:    10,
		CreatedAt:    time.Now().Add(-100 * time.Second), // Expired!
	})

	client := NewClient(ClientConfig{
		APIBaseURL:  apiServer.URL,
		OAuthClient: oauthClient,
		TokenStore:  store,
	})

	err := client.SendMeText(context.Background(), "Auto refresh test", "", "")
	assert.NoError(t, err)

	updatedToken, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "refreshed-token-999", updatedToken.AccessToken)
}
