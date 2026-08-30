package kakao

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultAuthBaseURL = "https://kauth.kakao.com"
	DefaultAPIBaseURL  = "https://kapi.kakao.com"

	// Common Kakao OAuth scopes
	ScopeTalkMessage = "talk_message"
	ScopeFriends     = "friends"
	ScopeProfile     = "profile_nickname"
)

// OAuthConfig contains OAuth credentials.
type OAuthConfig struct {
	AuthBaseURL  string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	HTTPClient   *http.Client
}

// OAuthClient handles Kakao OAuth2 token exchange and refresh.
type OAuthClient struct {
	config OAuthConfig
	client *http.Client
}

// NewOAuthClient creates a new Kakao OAuth client.
func NewOAuthClient(cfg OAuthConfig) *OAuthClient {
	if cfg.AuthBaseURL == "" {
		cfg.AuthBaseURL = DefaultAuthBaseURL
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: 15 * time.Second,
		}
	}
	return &OAuthClient{
		config: cfg,
		client: client,
	}
}

// GetAuthCodeURL builds the Kakao authorization URL for login.
func (c *OAuthClient) GetAuthCodeURL(scopes []string) string {
	u, _ := url.Parse(fmt.Sprintf("%s/oauth/authorize", c.config.AuthBaseURL))
	q := u.Query()
	q.Set("client_id", c.config.ClientID)
	q.Set("redirect_uri", c.config.RedirectURI)
	q.Set("response_type", "code")
	if len(scopes) > 0 {
		q.Set("scope", strings.Join(scopes, ","))
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// ExchangeToken exchanges an authorization code for access and refresh tokens.
func (c *OAuthClient) ExchangeToken(ctx context.Context, code string) (*TokenInfo, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", c.config.ClientID)
	data.Set("redirect_uri", c.config.RedirectURI)
	data.Set("code", code)
	if c.config.ClientSecret != "" {
		data.Set("client_secret", c.config.ClientSecret)
	}

	tokenURL := fmt.Sprintf("%s/oauth/token", c.config.AuthBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error() != "" {
			return nil, fmt.Errorf("token exchange failed (status %d): %w", resp.StatusCode, &apiErr)
		}
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var token TokenInfo
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("unmarshal token response: %w", err)
	}
	token.CreatedAt = time.Now()

	return &token, nil
}

// RefreshToken refreshes an expired access token using the refresh token.
func (c *OAuthClient) RefreshToken(ctx context.Context, refreshToken string) (*TokenInfo, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", c.config.ClientID)
	data.Set("refresh_token", refreshToken)
	if c.config.ClientSecret != "" {
		data.Set("client_secret", c.config.ClientSecret)
	}

	tokenURL := fmt.Sprintf("%s/oauth/token", c.config.AuthBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create refresh token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read refresh response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error() != "" {
			return nil, fmt.Errorf("token refresh failed (status %d): %w", resp.StatusCode, &apiErr)
		}
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var refreshed TokenInfo
	if err := json.Unmarshal(body, &refreshed); err != nil {
		return nil, fmt.Errorf("unmarshal refresh response: %w", err)
	}
	refreshed.CreatedAt = time.Now()
	// Kakao API may not return a new refresh token if the existing one has plenty of validity left.
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = refreshToken
	}

	return &refreshed, nil
}
