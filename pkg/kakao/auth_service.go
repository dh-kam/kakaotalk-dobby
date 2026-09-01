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

type authService struct {
	authBaseURL   string
	apiBaseURL    string
	clientID      string
	clientSecret  string
	redirectURI   string
	httpClient    *http.Client
	tokenProvider func(ctx context.Context) (string, error)
}

// NewAuthService creates a new AuthService instance.
func NewAuthService(authBaseURL, apiBaseURL, clientID, clientSecret, redirectURI string, httpClient *http.Client, tokenProvider func(ctx context.Context) (string, error)) AuthService {
	if authBaseURL == "" {
		authBaseURL = DefaultAuthBaseURL
	}
	if apiBaseURL == "" {
		apiBaseURL = DefaultAPIBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &authService{
		authBaseURL:   authBaseURL,
		apiBaseURL:    apiBaseURL,
		clientID:      clientID,
		clientSecret:  clientSecret,
		redirectURI:   redirectURI,
		httpClient:    httpClient,
		tokenProvider: tokenProvider,
	}
}

func (s *authService) GetAuthURL(scopes []string) string {
	return s.GetAuthURLWithState(scopes, "")
}

func (s *authService) GetAuthURLWithState(scopes []string, state string) string {
	u, _ := url.Parse(fmt.Sprintf("%s/oauth/authorize", s.authBaseURL))
	q := u.Query()
	q.Set("client_id", s.clientID)
	q.Set("redirect_uri", s.redirectURI)
	q.Set("response_type", "code")
	if state != "" {
		q.Set("state", state)
	}
	if len(scopes) > 0 {
		q.Set("scope", strings.Join(scopes, ","))
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *authService) ExchangeCode(ctx context.Context, code string) (*TokenInfo, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", s.clientID)
	data.Set("redirect_uri", s.redirectURI)
	data.Set("code", code)
	if s.clientSecret != "" {
		data.Set("client_secret", s.clientSecret)
	}

	tokenURL := fmt.Sprintf("%s/oauth/token", s.authBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := s.httpClient.Do(req)
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

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*TokenInfo, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", s.clientID)
	data.Set("refresh_token", refreshToken)
	if s.clientSecret != "" {
		data.Set("client_secret", s.clientSecret)
	}

	tokenURL := fmt.Sprintf("%s/oauth/token", s.authBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create refresh token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := s.httpClient.Do(req)
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
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = refreshToken
	}

	return &refreshed, nil
}

func (s *authService) GetAccessTokenInfo(ctx context.Context) (*AccessTokenInfo, error) {
	token, err := s.tokenProvider(ctx)
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/v1/user/access_token_info", s.apiBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create access token info request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute access token info request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error() != "" {
			return nil, fmt.Errorf("get token info failed (status %d): %w", resp.StatusCode, &apiErr)
		}
		return nil, fmt.Errorf("get token info failed with status %d: %s", resp.StatusCode, string(body))
	}

	var info AccessTokenInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("unmarshal token info: %w", err)
	}

	return &info, nil
}

func (s *authService) Logout(ctx context.Context) (int64, error) {
	token, err := s.tokenProvider(ctx)
	if err != nil {
		return 0, err
	}

	reqURL := fmt.Sprintf("%s/v1/user/logout", s.apiBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return 0, fmt.Errorf("create logout request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("execute logout request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read logout response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error() != "" {
			return 0, fmt.Errorf("logout failed (status %d): %w", resp.StatusCode, &apiErr)
		}
		return 0, fmt.Errorf("logout failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("unmarshal logout result: %w", err)
	}

	return result.ID, nil
}

func (s *authService) Unlink(ctx context.Context) (int64, error) {
	token, err := s.tokenProvider(ctx)
	if err != nil {
		return 0, err
	}

	reqURL := fmt.Sprintf("%s/v1/user/unlink", s.apiBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return 0, fmt.Errorf("create unlink request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("execute unlink request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read unlink response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error() != "" {
			return 0, fmt.Errorf("unlink failed (status %d): %w", resp.StatusCode, &apiErr)
		}
		return 0, fmt.Errorf("unlink failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("unmarshal unlink result: %w", err)
	}

	return result.ID, nil
}
