package ai

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// ProviderConfig holds settings to instantiate an AI provider.
type ProviderConfig struct {
	ProviderName string
	APIKey       string
	BaseURL      string
	Model        string
	HTTPClient   *http.Client
}

// NewProvider creates a Provider based on configuration.
func NewProvider(cfg ProviderConfig) (Provider, error) {
	name := strings.ToLower(strings.TrimSpace(cfg.ProviderName))
	if name == "" {
		if cfg.APIKey != "" {
			name = "openai"
		} else {
			name = "mock"
		}
	}

	switch name {
	case "openai":
		return NewOpenAIProvider(OpenAIConfig{
			BaseURL:      cfg.BaseURL,
			APIKey:       cfg.APIKey,
			DefaultModel: cfg.Model,
			HTTPClient:   cfg.HTTPClient,
		}), nil

	case "ollama":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434/v1"
		}
		model := cfg.Model
		if model == "" {
			model = "llama3"
		}
		return NewOpenAIProvider(OpenAIConfig{
			BaseURL:      baseURL,
			APIKey:       cfg.APIKey,
			DefaultModel: model,
			HTTPClient:   cfg.HTTPClient,
		}), nil

	case "groq":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://api.groq.com/openai/v1"
		}
		model := cfg.Model
		if model == "" {
			model = "llama-3.1-8b-instant"
		}
		return NewOpenAIProvider(OpenAIConfig{
			BaseURL:      baseURL,
			APIKey:       cfg.APIKey,
			DefaultModel: model,
			HTTPClient:   cfg.HTTPClient,
		}), nil

	case "deepseek":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://api.deepseek.com/v1"
		}
		model := cfg.Model
		if model == "" {
			model = "deepseek-chat"
		}
		return NewOpenAIProvider(OpenAIConfig{
			BaseURL:      baseURL,
			APIKey:       cfg.APIKey,
			DefaultModel: model,
			HTTPClient:   cfg.HTTPClient,
		}), nil

	case "gemini", "google":
		return NewGeminiProvider(GeminiConfig{
			APIKey:       cfg.APIKey,
			DefaultModel: cfg.Model,
			HTTPClient:   cfg.HTTPClient,
		}), nil

	case "claude", "anthropic":
		return NewClaudeProvider(ClaudeConfig{
			APIKey:       cfg.APIKey,
			DefaultModel: cfg.Model,
			HTTPClient:   cfg.HTTPClient,
		}), nil

	case "mock":
		return NewMockProvider(cfg.Model), nil

	default:
		return nil, fmt.Errorf("unsupported AI provider: %q (supported: openai, ollama, groq, deepseek, gemini, claude, mock)", cfg.ProviderName)
	}
}

// MockProvider is an offline mock provider for testing.
type MockProvider struct {
	model string
}

func NewMockProvider(model string) *MockProvider {
	if model == "" {
		model = "mock-model"
	}
	return &MockProvider{model: model}
}

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) GenerateResponse(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	lastMsg := ""
	if len(req.Messages) > 0 {
		lastMsg = req.Messages[len(req.Messages)-1].Content
	}
	return &CompletionResponse{
		Text:         fmt.Sprintf("🤖 [AI Mock Reply] \"%s\" 에 대한 AI 분석 답변입니다.", lastMsg),
		Model:        m.model,
		PromptTokens: 10,
		OutputTokens: 20,
	}, nil
}
