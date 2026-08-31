package ai

import "context"

// Provider defines the interface for interacting with LLMs.
type Provider interface {
	Name() string
	GenerateResponse(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
}

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// CompletionRequest holds parameters for generating an AI completion.
type CompletionRequest struct {
	Model        string        `json:"model,omitempty"`
	SystemPrompt string        `json:"system_prompt,omitempty"`
	Messages     []ChatMessage `json:"messages"`
	MaxTokens    int           `json:"max_tokens,omitempty"`
	Temperature  float64       `json:"temperature,omitempty"`
}

// CompletionResponse holds the generated text output and metadata.
type CompletionResponse struct {
	Text         string `json:"text"`
	Model        string `json:"model,omitempty"`
	PromptTokens int    `json:"prompt_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
}
