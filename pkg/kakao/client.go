package kakao

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client provides access to Kakao Talk REST APIs.
type Client struct {
	apiBaseURL  string
	oauthClient *OAuthClient
	tokenStore  TokenStore
	httpClient  *http.Client
}

// ClientConfig holds configuration for Kakao API client.
type ClientConfig struct {
	APIBaseURL  string
	OAuthClient *OAuthClient
	TokenStore  TokenStore
	HTTPClient  *http.Client
}

// NewClient creates a new Kakao API client.
func NewClient(cfg ClientConfig) *Client {
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = DefaultAPIBaseURL
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 15 * time.Second,
		}
	}
	return &Client{
		apiBaseURL:  cfg.APIBaseURL,
		oauthClient: cfg.OAuthClient,
		tokenStore:  cfg.TokenStore,
		httpClient:  httpClient,
	}
}

// GetTokenStore returns the configured token store.
func (c *Client) GetTokenStore() TokenStore {
	return c.tokenStore
}

// GetOAuthClient returns the configured OAuth client.
func (c *Client) GetOAuthClient() *OAuthClient {
	return c.oauthClient
}

// GetValidAccessToken returns an active access token, refreshing automatically if expired.
func (c *Client) GetValidAccessToken(ctx context.Context) (string, error) {
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

	if c.oauthClient == nil {
		return "", fmt.Errorf("token is expired and oauth client is not configured for automatic refresh")
	}

	if token.RefreshToken == "" {
		return "", fmt.Errorf("refresh token is empty; re-authentication is required")
	}

	refreshed, err := c.oauthClient.RefreshToken(ctx, token.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("auto refresh token failed: %w", err)
	}

	if err := c.tokenStore.Save(ctx, refreshed); err != nil {
		return "", fmt.Errorf("save refreshed token: %w", err)
	}

	return refreshed.AccessToken, nil
}

// GetMe retrieves the authenticated user profile.
func (c *Client) GetMe(ctx context.Context) (*UserProfile, error) {
	token, err := c.GetValidAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/v2/user/me", c.apiBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error() != "" {
			return nil, fmt.Errorf("get user failed (status %d): %w", resp.StatusCode, &apiErr)
		}
		return nil, fmt.Errorf("get user failed with status %d: %s", resp.StatusCode, string(body))
	}

	var profile UserProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, fmt.Errorf("unmarshal profile: %w", err)
	}

	return &profile, nil
}

// SendMeText sends a text template message to the authenticated user's KakaoTalk.
func (c *Client) SendMeText(ctx context.Context, text string, webURL string, buttonTitle string) error {
	tmpl := NewTextTemplate(text, webURL, buttonTitle)
	return c.SendMeTemplate(ctx, tmpl)
}

// SendMeFeed sends a feed template message to the authenticated user's KakaoTalk.
func (c *Client) SendMeFeed(ctx context.Context, feed *FeedTemplate) error {
	return c.SendMeTemplate(ctx, feed)
}

// SendMeList sends a list template message to the authenticated user's KakaoTalk.
func (c *Client) SendMeList(ctx context.Context, list *ListTemplate) error {
	return c.SendMeTemplate(ctx, list)
}

// SendMeTemplate sends a message to oneself using default template object.
func (c *Client) SendMeTemplate(ctx context.Context, template interface{}) error {
	tmplJSON, err := ToJSON(template)
	if err != nil {
		return fmt.Errorf("marshal template: %w", err)
	}

	token, err := c.GetValidAccessToken(ctx)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("template_object", tmplJSON)

	reqURL := fmt.Sprintf("%s/v2/api/talk/memo/default/send", c.apiBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create send request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error() != "" {
			return fmt.Errorf("send message failed (status %d): %w", resp.StatusCode, &apiErr)
		}
		return fmt.Errorf("send message failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SendMeCustom sends a message using a custom template ID defined in Kakao Developers.
func (c *Client) SendMeCustom(ctx context.Context, templateID int64, args map[string]string) error {
	token, err := c.GetValidAccessToken(ctx)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("template_id", strconv.FormatInt(templateID, 10))
	if len(args) > 0 {
		argsJSON, err := json.Marshal(args)
		if err != nil {
			return fmt.Errorf("marshal template args: %w", err)
		}
		form.Set("template_args", string(argsJSON))
	}

	reqURL := fmt.Sprintf("%s/v2/api/talk/memo/send", c.apiBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create custom send request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute custom send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error() != "" {
			return fmt.Errorf("custom send message failed (status %d): %w", resp.StatusCode, &apiErr)
		}
		return fmt.Errorf("custom send message failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetFriends retrieves the KakaoTalk friends list.
func (c *Client) GetFriends(ctx context.Context, offset, limit int) (*FriendsResponse, error) {
	token, err := c.GetValidAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	u, _ := url.Parse(fmt.Sprintf("%s/v1/api/talk/friends", c.apiBaseURL))
	q := u.Query()
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create friends request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute friends request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error() != "" {
			return nil, fmt.Errorf("get friends failed (status %d): %w", resp.StatusCode, &apiErr)
		}
		return nil, fmt.Errorf("get friends failed with status %d: %s", resp.StatusCode, string(body))
	}

	var friends FriendsResponse
	if err := json.Unmarshal(body, &friends); err != nil {
		return nil, fmt.Errorf("unmarshal friends: %w", err)
	}

	return &friends, nil
}

// SendFriendsText sends a text message to specified Kakao friends.
func (c *Client) SendFriendsText(ctx context.Context, receiverUUIDs []string, text string, webURL string, buttonTitle string) (*MessageResult, error) {
	tmpl := NewTextTemplate(text, webURL, buttonTitle)
	return c.SendFriendsTemplate(ctx, receiverUUIDs, tmpl)
}

// SendFriendsTemplate sends a default template message to specified Kakao friends.
func (c *Client) SendFriendsTemplate(ctx context.Context, receiverUUIDs []string, template interface{}) (*MessageResult, error) {
	if len(receiverUUIDs) == 0 {
		return nil, fmt.Errorf("receiver UUID list cannot be empty")
	}

	uuidsJSON, err := json.Marshal(receiverUUIDs)
	if err != nil {
		return nil, fmt.Errorf("marshal receiver uuids: %w", err)
	}

	tmplJSON, err := ToJSON(template)
	if err != nil {
		return nil, fmt.Errorf("marshal template: %w", err)
	}

	token, err := c.GetValidAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("receiver_uuids", string(uuidsJSON))
	form.Set("template_object", tmplJSON)

	reqURL := fmt.Sprintf("%s/v1/api/talk/friends/message/default/send", c.apiBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create friends send request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute friends send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error() != "" {
			return nil, fmt.Errorf("friends send failed (status %d): %w", resp.StatusCode, &apiErr)
		}
		return nil, fmt.Errorf("friends send failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result MessageResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal send result: %w", err)
	}

	return &result, nil
}

// SendFriendsCustom sends a custom template message to specified Kakao friends.
func (c *Client) SendFriendsCustom(ctx context.Context, receiverUUIDs []string, templateID int64, args map[string]string) (*MessageResult, error) {
	if len(receiverUUIDs) == 0 {
		return nil, fmt.Errorf("receiver UUID list cannot be empty")
	}

	uuidsJSON, err := json.Marshal(receiverUUIDs)
	if err != nil {
		return nil, fmt.Errorf("marshal receiver uuids: %w", err)
	}

	token, err := c.GetValidAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("receiver_uuids", string(uuidsJSON))
	form.Set("template_id", strconv.FormatInt(templateID, 10))
	if len(args) > 0 {
		argsJSON, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("marshal template args: %w", err)
		}
		form.Set("template_args", string(argsJSON))
	}

	reqURL := fmt.Sprintf("%s/v1/api/talk/friends/message/send", c.apiBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create friends custom send request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute friends custom send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error() != "" {
			return nil, fmt.Errorf("friends custom send failed (status %d): %w", resp.StatusCode, &apiErr)
		}
		return nil, fmt.Errorf("friends custom send failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result MessageResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal send result: %w", err)
	}

	return &result, nil
}
