package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type geminiProvider struct {
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

// GeminiConfig holds configuration for Google Gemini provider.
type GeminiConfig struct {
	APIKey       string
	DefaultModel string
	HTTPClient   *http.Client
}

// NewGeminiProvider creates an AI provider for Google Gemini.
func NewGeminiProvider(cfg GeminiConfig) Provider {
	model := cfg.DefaultModel
	if model == "" {
		model = "gemini-1.5-flash"
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &geminiProvider{
		apiKey:       cfg.APIKey,
		defaultModel: model,
		httpClient:   client,
	}
}

func (p *geminiProvider) Name() string {
	return "gemini"
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func (p *geminiProvider) GenerateResponse(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	contents := make([]geminiContent, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	gReq := geminiRequest{
		Contents: contents,
	}
	if req.SystemPrompt != "" {
		gReq.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: req.SystemPrompt}},
		}
	}

	reqBytes, err := json.Marshal(gReq)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini request: %w", err)
	}

	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent",
		url.PathEscape(model))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("create gemini request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("x-goog-api-key", p.apiKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute gemini request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read gemini response body: %w", err)
	}

	var gResp geminiResponse
	if err := json.Unmarshal(respBody, &gResp); err != nil {
		return nil, fmt.Errorf("unmarshal gemini response: %w", err)
	}

	if gResp.Error != nil {
		return nil, fmt.Errorf("gemini error: %s (code %d)", gResp.Error.Message, gResp.Error.Code)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	if len(gResp.Candidates) == 0 || len(gResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response candidates returned from gemini")
	}

	return &CompletionResponse{
		Text:         gResp.Candidates[0].Content.Parts[0].Text,
		Model:        model,
		PromptTokens: gResp.UsageMetadata.PromptTokenCount,
		OutputTokens: gResp.UsageMetadata.CandidatesTokenCount,
	}, nil
}
