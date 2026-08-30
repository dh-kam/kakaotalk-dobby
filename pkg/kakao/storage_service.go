package kakao

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type storageService struct {
	apiBaseURL    string
	httpClient    *http.Client
	tokenProvider func(ctx context.Context) (string, error)
}

// NewStorageService creates a new StorageService instance.
func NewStorageService(apiBaseURL string, httpClient *http.Client, tokenProvider func(ctx context.Context) (string, error)) StorageService {
	if apiBaseURL == "" {
		apiBaseURL = DefaultAPIBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &storageService{
		apiBaseURL:    apiBaseURL,
		httpClient:    httpClient,
		tokenProvider: tokenProvider,
	}
}

func (s *storageService) UploadImage(ctx context.Context, reader io.Reader, filename string) (*UploadedImageInfo, error) {
	token, err := s.tokenProvider(ctx)
	if err != nil {
		return nil, err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if filename == "" {
		filename = "image.png"
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}

	if _, err := io.Copy(part, reader); err != nil {
		return nil, fmt.Errorf("copy image data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	reqURL := fmt.Sprintf("%s/v2/api/talk/message/image/upload", s.apiBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, &body)
	if err != nil {
		return nil, fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute upload request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read upload response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Error() != "" {
			return nil, fmt.Errorf("image upload failed (status %d): %w", resp.StatusCode, &apiErr)
		}
		return nil, fmt.Errorf("image upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var info UploadedImageInfo
	if err := json.Unmarshal(respBody, &info); err != nil {
		return nil, fmt.Errorf("unmarshal upload response: %w", err)
	}

	return &info, nil
}

func (s *storageService) ScrapImage(ctx context.Context, imageURL string) (*UploadedImageInfo, error) {
	token, err := s.tokenProvider(ctx)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("image_url", imageURL)

	reqURL := fmt.Sprintf("%s/v2/api/talk/message/image/scrap", s.apiBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create scrap image request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute scrap image request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read scrap image response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Error() != "" {
			return nil, fmt.Errorf("image scrap failed (status %d): %w", resp.StatusCode, &apiErr)
		}
		return nil, fmt.Errorf("image scrap failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var info UploadedImageInfo
	if err := json.Unmarshal(respBody, &info); err != nil {
		return nil, fmt.Errorf("unmarshal scrap response: %w", err)
	}

	return &info, nil
}

func (s *storageService) DeleteImage(ctx context.Context, imageURL string) error {
	token, err := s.tokenProvider(ctx)
	if err != nil {
		return err
	}

	u, _ := url.Parse(fmt.Sprintf("%s/v2/api/talk/message/image/delete", s.apiBaseURL))
	q := u.Query()
	q.Set("image_url", imageURL)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		return fmt.Errorf("create delete image request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute delete image request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read delete response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Error() != "" {
			return fmt.Errorf("image delete failed (status %d): %w", resp.StatusCode, &apiErr)
		}
		return fmt.Errorf("image delete failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
