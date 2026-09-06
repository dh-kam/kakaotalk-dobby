package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type openAIProvider struct {
	name         string
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

// OpenAIConfig holds configuration for OpenAI and OpenAI-compatible providers.
type OpenAIConfig struct {
	ProviderName string
	BaseURL      string
	APIKey       string
	DefaultModel string
	HTTPClient   *http.Client
}

// NewOpenAIProvider creates an AI provider for OpenAI and compatible APIs (Ollama, Groq, DeepSeek, vLLM).
func NewOpenAIProvider(cfg OpenAIConfig) Provider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := cfg.DefaultModel
	if model == "" {
		model = "gpt-4o-mini"
	}
	name := cfg.ProviderName
	if name == "" {
		name = "openai"
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &openAIProvider{
		name:         name,
		baseURL:      baseURL,
		apiKey:       cfg.APIKey,
		defaultModel: model,
		httpClient:   client,
	}
}

func (p *openAIProvider) Model() string {
	return p.defaultModel
}

func (p *openAIProvider) Name() string {
	return p.name
}

type openAIChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error json.RawMessage `json:"error,omitempty"`
}

// describeProviderError renders OpenAI-compatible error payloads, which appear
// either as an object ({"error":{"message":...}}) or a bare string (xAI).
func describeProviderError(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}

	var structured struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	}
	if err := json.Unmarshal(raw, &structured); err == nil && structured.Message != "" {
		if structured.Type != "" {
			return fmt.Sprintf("%s (%s)", structured.Message, structured.Type)
		}
		return structured.Message
	}

	var bare string
	if err := json.Unmarshal(raw, &bare); err == nil {
		return bare
	}
	return trimmed
}

func (p *openAIProvider) GenerateResponse(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	messages := make([]ChatMessage, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, ChatMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}
	messages = append(messages, req.Messages...)

	chatReq := openAIChatRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal openai request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/chat/completions", p.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create openai request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute openai request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read openai response body: %w", err)
	}

	var chatResp openAIChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal openai response: %w", err)
	}

	if msg := describeProviderError(chatResp.Error); msg != "" {
		return nil, fmt.Errorf("%s error: %s", p.name, msg)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no completion choices returned from openai")
	}

	return &CompletionResponse{
		Text:         chatResp.Choices[0].Message.Content,
		Model:        model,
		PromptTokens: chatResp.Usage.PromptTokens,
		OutputTokens: chatResp.Usage.CompletionTokens,
	}, nil
}
