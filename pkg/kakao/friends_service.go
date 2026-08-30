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

type friendsService struct {
	apiBaseURL    string
	httpClient    *http.Client
	tokenProvider func(ctx context.Context) (string, error)
}

// NewFriendsService creates a new FriendsService instance.
func NewFriendsService(apiBaseURL string, httpClient *http.Client, tokenProvider func(ctx context.Context) (string, error)) FriendsService {
	if apiBaseURL == "" {
		apiBaseURL = DefaultAPIBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &friendsService{
		apiBaseURL:    apiBaseURL,
		httpClient:    httpClient,
		tokenProvider: tokenProvider,
	}
}

func (s *friendsService) GetFriends(ctx context.Context, opts FriendsQueryOptions) (*FriendsResponse, error) {
	token, err := s.tokenProvider(ctx)
	if err != nil {
		return nil, err
	}

	u, _ := url.Parse(fmt.Sprintf("%s/v1/api/talk/friends", s.apiBaseURL))
	q := u.Query()
	if opts.Offset > 0 {
		q.Set("offset", strconv.Itoa(opts.Offset))
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Order != "" {
		q.Set("order", opts.Order)
	}
	if opts.FriendOrder != "" {
		q.Set("friend_order", opts.FriendOrder)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create friends request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := s.httpClient.Do(req)
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

func (s *friendsService) SendText(ctx context.Context, receiverUUIDs []string, req TextMessageRequest) (*MessageResult, error) {
	tmpl := NewTextTemplate(req.Text, req.WebURL, req.ButtonTitle)
	if req.MobileURL != "" {
		tmpl.Link.MobileWebURL = req.MobileURL
	}
	return s.sendDefault(ctx, receiverUUIDs, tmpl)
}

func (s *friendsService) SendFeed(ctx context.Context, receiverUUIDs []string, feed FeedTemplate) (*MessageResult, error) {
	return s.sendDefault(ctx, receiverUUIDs, feed)
}

func (s *friendsService) SendList(ctx context.Context, receiverUUIDs []string, list ListTemplate) (*MessageResult, error) {
	return s.sendDefault(ctx, receiverUUIDs, list)
}

func (s *friendsService) SendCommerce(ctx context.Context, receiverUUIDs []string, commerce CommerceTemplate) (*MessageResult, error) {
	return s.sendDefault(ctx, receiverUUIDs, commerce)
}

func (s *friendsService) SendLocation(ctx context.Context, receiverUUIDs []string, location LocationTemplate) (*MessageResult, error) {
	return s.sendDefault(ctx, receiverUUIDs, location)
}

func (s *friendsService) SendScrap(ctx context.Context, receiverUUIDs []string, requestURL string, templateID int64, args map[string]string) (*MessageResult, error) {
	if len(receiverUUIDs) == 0 {
		return nil, fmt.Errorf("receiver UUID list cannot be empty")
	}

	uuidsJSON, err := json.Marshal(receiverUUIDs)
	if err != nil {
		return nil, fmt.Errorf("marshal receiver uuids: %w", err)
	}

	token, err := s.tokenProvider(ctx)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("receiver_uuids", string(uuidsJSON))
	form.Set("request_url", requestURL)
	if templateID > 0 {
		form.Set("template_id", strconv.FormatInt(templateID, 10))
	}
	if len(args) > 0 {
		argsJSON, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("marshal template args: %w", err)
		}
		form.Set("template_args", string(argsJSON))
	}

	reqURL := fmt.Sprintf("%s/v1/api/talk/friends/message/scrap/send", s.apiBaseURL)
	return s.postForm(ctx, reqURL, token, form)
}

func (s *friendsService) SendCustom(ctx context.Context, receiverUUIDs []string, templateID int64, args map[string]string) (*MessageResult, error) {
	if len(receiverUUIDs) == 0 {
		return nil, fmt.Errorf("receiver UUID list cannot be empty")
	}

	uuidsJSON, err := json.Marshal(receiverUUIDs)
	if err != nil {
		return nil, fmt.Errorf("marshal receiver uuids: %w", err)
	}

	token, err := s.tokenProvider(ctx)
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

	reqURL := fmt.Sprintf("%s/v1/api/talk/friends/message/send", s.apiBaseURL)
	return s.postForm(ctx, reqURL, token, form)
}

func (s *friendsService) sendDefault(ctx context.Context, receiverUUIDs []string, template interface{}) (*MessageResult, error) {
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

	token, err := s.tokenProvider(ctx)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("receiver_uuids", string(uuidsJSON))
	form.Set("template_object", tmplJSON)

	reqURL := fmt.Sprintf("%s/v1/api/talk/friends/message/default/send", s.apiBaseURL)
	return s.postForm(ctx, reqURL, token, form)
}

func (s *friendsService) postForm(ctx context.Context, reqURL, token string, form url.Values) (*MessageResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create friends send request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := s.httpClient.Do(req)
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
