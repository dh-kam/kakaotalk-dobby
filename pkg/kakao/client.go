package kakao

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	DefaultAuthBaseURL = "https://kauth.kakao.com"
	DefaultAPIBaseURL  = "https://kapi.kakao.com"

	ScopeTalkMessage = "talk_message"
	ScopeFriends     = "friends"
	ScopeProfile     = "profile_nickname"
)

type defaultClient struct {
	authBaseURL  string
	apiBaseURL   string
	clientID     string
	clientSecret string
	redirectURI  string
	tokenStore   TokenStore
	httpClient   *http.Client

	authService    AuthService
	userService    UserService
	memoService    MemoService
	friendsService FriendsService
	storageService StorageService

	mu sync.Mutex
}

// ClientConfig holds configuration for the Kakao client.
type ClientConfig struct {
	AuthBaseURL  string
	APIBaseURL   string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenStore   TokenStore
	HTTPClient   *http.Client
}

// NewClient creates a new Client interface implementation.
func NewClient(cfg ClientConfig) Client {
	if cfg.AuthBaseURL == "" {
		cfg.AuthBaseURL = DefaultAuthBaseURL
	}
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = DefaultAPIBaseURL
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	tokenStore := cfg.TokenStore
	if tokenStore == nil {
		tokenStore = NewFileTokenStore(DefaultTokenPath())
	}

	c := &defaultClient{
		authBaseURL:  cfg.AuthBaseURL,
		apiBaseURL:   cfg.APIBaseURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		redirectURI:  cfg.RedirectURI,
		tokenStore:   tokenStore,
		httpClient:   httpClient,
	}

	tokenProvider := c.GetValidAccessToken

	c.authService = NewAuthService(c.authBaseURL, c.apiBaseURL, c.clientID, c.clientSecret, c.redirectURI, c.httpClient, tokenProvider)
	c.userService = NewUserService(c.apiBaseURL, c.httpClient, tokenProvider)
	c.memoService = NewMemoService(c.apiBaseURL, c.httpClient, tokenProvider)
	c.friendsService = NewFriendsService(c.apiBaseURL, c.httpClient, tokenProvider)
	c.storageService = NewStorageService(c.apiBaseURL, c.httpClient, tokenProvider)

	return c
}

func (c *defaultClient) Auth() AuthService {
	return c.authService
}

func (c *defaultClient) User() UserService {
	return c.userService
}

func (c *defaultClient) Memo() MemoService {
	return c.memoService
}

func (c *defaultClient) Friends() FriendsService {
	return c.friendsService
}

func (c *defaultClient) Storage() StorageService {
	return c.storageService
}

func (c *defaultClient) GetTokenStore() TokenStore {
	return c.tokenStore
}

func (c *defaultClient) GetValidAccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tokenStore == nil {
		return "", fmt.Errorf("token store is not configured")
	}

	token, err := c.tokenStore.Load(ctx)
	if err != nil {
		return "", fmt.Errorf("load token: %w", err)
	}

	if !token.IsExpired() {
		return token.AccessToken, nil
	}

	if c.authService == nil || c.clientID == "" {
		return "", fmt.Errorf("token is expired and auth service is not configured for refresh")
	}

	if token.RefreshToken == "" {
		return "", fmt.Errorf("refresh token is empty; re-authentication is required")
	}

	refreshed, err := c.authService.RefreshToken(ctx, token.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("auto refresh token failed: %w", err)
	}

	if err := c.tokenStore.Save(ctx, refreshed); err != nil {
		return "", fmt.Errorf("save refreshed token: %w", err)
	}

	return refreshed.AccessToken, nil
}
