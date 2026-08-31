package agent

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// ProviderOptions holds unified configuration to build any supported LLM provider.
type ProviderOptions struct {
	ProviderName  string
	Model         string
	Project       string // GCP project for Vertex AI
	Location      string // GCP region for Vertex AI (e.g. us-central1)
	Region        string // AWS region for Bedrock (e.g. us-east-1)
	BearerToken   string // GCP access token or Bedrock bearer token
	APIKey        string // Optional API Key
	AccessKeyID   string // AWS Access Key for Bedrock
	SecretKey     string // AWS Secret Key for Bedrock
	CustomBaseURL string // For mock/testing or proxy URLs
	HTTPClient    *http.Client
}

// NewLLMProvider constructs an LLMProvider instance based on provider name.
func NewLLMProvider(opts ProviderOptions) (LLMProvider, error) {
	name := strings.ToLower(strings.TrimSpace(opts.ProviderName))

	switch name {
	case "vertex", "vertexai", "google-vertex", "gcp":
		return NewVertexProvider(VertexConfig{
			Project:       opts.Project,
			Location:      opts.Location,
			Model:         opts.Model,
			BearerToken:   opts.BearerToken,
			APIKey:        opts.APIKey,
			CustomBaseURL: opts.CustomBaseURL,
			HTTPClient:    opts.HTTPClient,
		}), nil

	case "bedrock", "aws-bedrock", "aws":
		return NewBedrockProvider(BedrockConfig{
			Region:        opts.Region,
			ModelID:       opts.Model,
			BearerToken:   opts.BearerToken,
			AccessKeyID:   opts.AccessKeyID,
			SecretKey:     opts.SecretKey,
			CustomBaseURL: opts.CustomBaseURL,
			HTTPClient:    opts.HTTPClient,
		}), nil

	case "mock":
		return NewMockLLMProvider(opts.Model), nil

	default:
		return nil, fmt.Errorf("unsupported agent LLM provider: %q (supported: vertex, bedrock, mock)", opts.ProviderName)
	}
}

// MockLLMProvider is a mock provider for agent unit testing.
type MockLLMProvider struct {
	model        string
	CustomAnswer string
	ToolCallPlan []ToolCall
	callCount    int
}

func NewMockLLMProvider(model string) *MockLLMProvider {
	if model == "" {
		model = "mock-model"
	}
	return &MockLLMProvider{model: model}
}

func (m *MockLLMProvider) Name() string {
	return "mock"
}

func (m *MockLLMProvider) Generate(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	return &CompletionResponse{
		Text:  "Mock response: " + m.CustomAnswer,
		Model: m.model,
		Usage: TokenUsage{InputTokens: 10, OutputTokens: 10, TotalTokens: 20},
	}, nil
}

func (m *MockLLMProvider) GenerateWithTools(ctx context.Context, req ToolCompletionRequest) (*ToolCompletionResponse, error) {
	m.callCount++
	// If tool call plan exists and this is the first turn, trigger tool calls
	if len(m.ToolCallPlan) > 0 && m.callCount == 1 {
		return &ToolCompletionResponse{
			Text:      "I will call tools to solve this.",
			ToolCalls: m.ToolCallPlan,
			Model:     m.model,
			Usage:     TokenUsage{InputTokens: 15, OutputTokens: 15, TotalTokens: 30},
		}, nil
	}

	ans := m.CustomAnswer
	if ans == "" {
		ans = "Final mock answer after reasoning."
	}

	return &ToolCompletionResponse{
		Text:  ans,
		Model: m.model,
		Usage: TokenUsage{InputTokens: 20, OutputTokens: 20, TotalTokens: 40},
	}, nil
}
