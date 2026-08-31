package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type claudeProvider struct {
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

// ClaudeConfig holds configuration for Anthropic Claude provider.
type ClaudeConfig struct {
	APIKey       string
	DefaultModel string
	HTTPClient   *http.Client
}

// NewClaudeProvider creates an AI provider for Anthropic Claude.
func NewClaudeProvider(cfg ClaudeConfig) Provider {
	model := cfg.DefaultModel
	if model == "" {
		model = "claude-3-5-haiku-20241022"
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &claudeProvider{
		apiKey:       cfg.APIKey,
		defaultModel: model,
		httpClient:   client,
	}
}

func (p *claudeProvider) Name() string {
	return "claude"
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *claudeProvider) GenerateResponse(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1000
	}

	messages := make([]claudeMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			continue
		}
		messages = append(messages, claudeMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	cReq := claudeRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    req.SystemPrompt,
		Messages:  messages,
	}

	reqBytes, err := json.Marshal(cReq)
	if err != nil {
		return nil, fmt.Errorf("marshal claude request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("create claude request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute claude request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read claude response body: %w", err)
	}

	var cResp claudeResponse
	if err := json.Unmarshal(respBody, &cResp); err != nil {
		return nil, fmt.Errorf("unmarshal claude response: %w", err)
	}

	if cResp.Error != nil {
		return nil, fmt.Errorf("claude error: %s (%s)", cResp.Error.Message, cResp.Error.Type)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	if len(cResp.Content) == 0 {
		return nil, fmt.Errorf("no content returned from claude")
	}

	return &CompletionResponse{
		Text:         cResp.Content[0].Text,
		Model:        model,
		PromptTokens: cResp.Usage.InputTokens,
		OutputTokens: cResp.Usage.OutputTokens,
	}, nil
}
