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

type memoService struct {
	apiBaseURL    string
	httpClient    *http.Client
	tokenProvider func(ctx context.Context) (string, error)
}

// NewMemoService creates a new MemoService instance.
func NewMemoService(apiBaseURL string, httpClient *http.Client, tokenProvider func(ctx context.Context) (string, error)) MemoService {
	if apiBaseURL == "" {
		apiBaseURL = DefaultAPIBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &memoService{
		apiBaseURL:    apiBaseURL,
		httpClient:    httpClient,
		tokenProvider: tokenProvider,
	}
}

func (s *memoService) SendText(ctx context.Context, req TextMessageRequest) error {
	tmpl := NewTextTemplate(req.Text, req.WebURL, req.ButtonTitle)
	if req.MobileURL != "" {
		tmpl.Link.MobileWebURL = req.MobileURL
	}
	return s.sendDefault(ctx, tmpl)
}

func (s *memoService) SendFeed(ctx context.Context, feed FeedTemplate) error {
	return s.sendDefault(ctx, feed)
}

func (s *memoService) SendList(ctx context.Context, list ListTemplate) error {
	return s.sendDefault(ctx, list)
}

func (s *memoService) SendCommerce(ctx context.Context, commerce CommerceTemplate) error {
	return s.sendDefault(ctx, commerce)
}

func (s *memoService) SendLocation(ctx context.Context, location LocationTemplate) error {
	return s.sendDefault(ctx, location)
}

func (s *memoService) SendScrap(ctx context.Context, requestURL string, templateID int64, args map[string]string) error {
	token, err := s.tokenProvider(ctx)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("request_url", requestURL)
	if templateID > 0 {
		form.Set("template_id", strconv.FormatInt(templateID, 10))
	}
	if len(args) > 0 {
		argsJSON, err := json.Marshal(args)
		if err != nil {
			return fmt.Errorf("marshal template args: %w", err)
		}
		form.Set("template_args", string(argsJSON))
	}

	reqURL := fmt.Sprintf("%s/v2/api/talk/memo/scrap/send", s.apiBaseURL)
	return s.postForm(ctx, reqURL, token, form)
}

func (s *memoService) SendCustom(ctx context.Context, templateID int64, args map[string]string) error {
	token, err := s.tokenProvider(ctx)
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

	reqURL := fmt.Sprintf("%s/v2/api/talk/memo/send", s.apiBaseURL)
	return s.postForm(ctx, reqURL, token, form)
}

func (s *memoService) sendDefault(ctx context.Context, template interface{}) error {
	tmplJSON, err := ToJSON(template)
	if err != nil {
		return fmt.Errorf("marshal template: %w", err)
	}

	token, err := s.tokenProvider(ctx)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("template_object", tmplJSON)

	reqURL := fmt.Sprintf("%s/v2/api/talk/memo/default/send", s.apiBaseURL)
	return s.postForm(ctx, reqURL, token, form)
}

func (s *memoService) postForm(ctx context.Context, reqURL, token string, form url.Values) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create memo request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute memo request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read memo response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error() != "" {
			return fmt.Errorf("memo request failed (status %d): %w", resp.StatusCode, &apiErr)
		}
		return fmt.Errorf("memo request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
