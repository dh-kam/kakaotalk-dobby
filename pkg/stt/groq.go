package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultGroqEndpoint = "https://api.groq.com/openai/v1/audio/transcriptions"
	DefaultGroqModel    = "whisper-large-v3-turbo"
)

// GroqConfig holds configuration for the Groq Cloud STT client.
type GroqConfig struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
}

type groqTranscriber struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// NewGroqTranscriber creates a new Groq Cloud Whisper transcriber.
func NewGroqTranscriber(cfg GroqConfig) Transcriber {
	model := cfg.Model
	if model == "" {
		model = DefaultGroqModel
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultGroqEndpoint
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	return &groqTranscriber{
		apiKey:     strings.TrimSpace(cfg.APIKey),
		model:      model,
		baseURL:    baseURL,
		httpClient: client,
	}
}

func (g *groqTranscriber) Name() string {
	return "groq"
}

// APIError represents an error response from Groq API.
type APIError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	return fmt.Sprintf("groq stt error (HTTP %d): %s", e.StatusCode, e.Message)
}

func (g *groqTranscriber) Transcribe(ctx context.Context, audioPath string, opts TranscribeOptions) (*TranscribeResult, error) {
	if g.apiKey == "" {
		return nil, ErrInvalidAPIKey
	}

	file, err := os.Open(audioPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrAudioFileNotFound
		}
		return nil, fmt.Errorf("open audio file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat audio file: %w", err)
	}
	if info.Size() > 25*1024*1024 {
		return nil, ErrAudioTooLarge
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, fmt.Errorf("create multipart file part: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("write audio to multipart: %w", err)
	}

	_ = writer.WriteField("model", g.model)

	lang := opts.Language
	if lang == "" {
		lang = "ko"
	}
	_ = writer.WriteField("language", lang)

	respFormat := opts.ResponseFormat
	if respFormat == "" {
		respFormat = "text"
	}
	_ = writer.WriteField("response_format", respFormat)

	if opts.Prompt != "" {
		_ = writer.WriteField("prompt", opts.Prompt)
	}
	if opts.Temperature > 0 {
		_ = writer.WriteField("temperature", fmt.Sprintf("%.2f", opts.Temperature))
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "POST", g.baseURL, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("groq http request failed: %w", err)
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var retryAfter time.Duration
		if raHeader := resp.Header.Get("Retry-After"); raHeader != "" {
			if secs, err := strconv.Atoi(raHeader); err == nil && secs > 0 {
				retryAfter = time.Duration(secs) * time.Second
			}
		}
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    strings.TrimSpace(string(respBytes)),
			RetryAfter: retryAfter,
		}
	}

	rawText := strings.TrimSpace(string(respBytes))
	// If json format was returned, parse the text field
	if strings.HasPrefix(rawText, "{") && strings.Contains(rawText, `"text"`) {
		var parsed struct {
			Text string `json:"text"`
		}
		if jsonErr := json.Unmarshal(respBytes, &parsed); jsonErr == nil && parsed.Text != "" {
			rawText = strings.TrimSpace(parsed.Text)
		}
	}

	return &TranscribeResult{
		Text:     rawText,
		Duration: elapsed,
		Provider: "groq",
		Model:    g.model,
		Fallback: false,
	}, nil
}
