package kakao

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type userService struct {
	apiBaseURL    string
	httpClient    *http.Client
	tokenProvider func(ctx context.Context) (string, error)
}

// NewUserService creates a new UserService instance.
func NewUserService(apiBaseURL string, httpClient *http.Client, tokenProvider func(ctx context.Context) (string, error)) UserService {
	if apiBaseURL == "" {
		apiBaseURL = DefaultAPIBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &userService{
		apiBaseURL:    apiBaseURL,
		httpClient:    httpClient,
		tokenProvider: tokenProvider,
	}
}

func (s *userService) GetMe(ctx context.Context) (*UserProfile, error) {
	token, err := s.tokenProvider(ctx)
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/v2/user/me", s.apiBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := s.httpClient.Do(req)
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

func (s *userService) GetShippingAddresses(ctx context.Context, fromUpdated, pageSize int) (*ShippingAddressesResponse, error) {
	token, err := s.tokenProvider(ctx)
	if err != nil {
		return nil, err
	}

	u, _ := url.Parse(fmt.Sprintf("%s/v1/user/shipping_address", s.apiBaseURL))
	q := u.Query()
	if fromUpdated > 0 {
		q.Set("from_updated_at", strconv.Itoa(fromUpdated))
	}
	if pageSize > 0 {
		q.Set("page_size", strconv.Itoa(pageSize))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create shipping address request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute shipping address request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read shipping address response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error() != "" {
			return nil, fmt.Errorf("get shipping address failed (status %d): %w", resp.StatusCode, &apiErr)
		}
		return nil, fmt.Errorf("get shipping address failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result ShippingAddressesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal shipping address response: %w", err)
	}

	return &result, nil
}
