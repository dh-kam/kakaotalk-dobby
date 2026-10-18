package kakao

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenKeeper_DefaultConfig(t *testing.T) {
	keeper := NewTokenKeeper(TokenKeeperConfig{})
	assert.Equal(t, DefaultTokenKeeperInterval, keeper.cfg.Interval)
	assert.NotNil(t, keeper.cfg.Logger)
}

func TestTokenKeeper_StartStop(t *testing.T) {
	keeper := NewTokenKeeper(TokenKeeperConfig{
		Interval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := keeper.Start(ctx)
	require.NoError(t, err)

	// Second start should fail
	err = keeper.Start(ctx)
	assert.Error(t, err)

	keeper.Stop()
	assert.False(t, keeper.running)
}

func TestTokenKeeper_RefreshNow(t *testing.T) {
	var refreshCallCount int32

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oauth/token", r.URL.Path)
		assert.Equal(t, "refresh_token", r.FormValue("grant_type"))
		assert.Equal(t, "initial-refresh-token", r.FormValue("refresh_token"))

		atomic.AddInt32(&refreshCallCount, 1)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"access_token": "new-access-token-123",
			"token_type": "bearer",
			"refresh_token": "updated-refresh-token-456",
			"expires_in": 21599,
			"refresh_token_expires_in": 5183999
		}`))
	}))
	defer authServer.Close()

	store := NewMemoryTokenStore(&TokenInfo{
		AccessToken:  "old-access-token",
		RefreshToken: "initial-refresh-token",
		ExpiresIn:    3600,
		CreatedAt:    time.Now().Add(-2 * time.Hour),
	})

	client := NewClient(ClientConfig{
		AuthBaseURL: authServer.URL,
		ClientID:    "test-client-id",
		TokenStore:  store,
	})

	keeper := NewTokenKeeper(TokenKeeperConfig{
		Client:   client,
		Interval: 1 * time.Hour,
	})

	err := keeper.RefreshNow(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&refreshCallCount))

	// Verify store updated with new tokens
	updated, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "new-access-token-123", updated.AccessToken)
	assert.Equal(t, "updated-refresh-token-456", updated.RefreshToken)
}

func TestTokenKeeper_PeriodicRefresh(t *testing.T) {
	var refreshCallCount int32

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&refreshCallCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"access_token": "refreshed-tok",
			"token_type": "bearer",
			"expires_in": 21599
		}`))
	}))
	defer authServer.Close()

	store := NewMemoryTokenStore(&TokenInfo{
		AccessToken:  "initial-access",
		RefreshToken: "initial-refresh",
	})

	client := NewClient(ClientConfig{
		AuthBaseURL: authServer.URL,
		ClientID:    "test-client-id",
		TokenStore:  store,
	})

	keeper := NewTokenKeeper(TokenKeeperConfig{
		Client:       client,
		Interval:     20 * time.Millisecond,
		CheckOnStart: true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := keeper.Start(ctx)
	require.NoError(t, err)

	// Wait for at least 2 ticks
	assert.Eventually(t, func() bool {
		return atomic.LoadInt32(&refreshCallCount) >= 2
	}, 500*time.Millisecond, 20*time.Millisecond)

	keeper.Stop()
}

func TestTokenKeeper_EmptyTokenHandled(t *testing.T) {
	// Store with empty refresh token
	store := NewMemoryTokenStore(&TokenInfo{
		AccessToken: "some-access",
	})

	client := NewClient(ClientConfig{
		TokenStore: store,
	})

	keeper := NewTokenKeeper(TokenKeeperConfig{
		Client: client,
	})

	err := keeper.RefreshNow(context.Background())
	assert.NoError(t, err)
}
