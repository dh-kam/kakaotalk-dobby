package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/samber/lo"
)

type vertexProvider struct {
	project      string
	location     string
	model        string
	bearerToken  string
	apiKey       string
	customBaseURL string
	httpClient   *http.Client
}

// VertexConfig holds configuration for Google Cloud Vertex AI.
type VertexConfig struct {
	Project       string
	Location      string
	Model         string
	BearerToken   string // GCP OAuth2 Access Token (e.g. from gcloud auth print-access-token)
	APIKey        string // Optional API Key for Vertex Express mode
	CustomBaseURL string // For testing or custom proxy endpoints
	HTTPClient    *http.Client
}

// NewVertexProvider creates a new Google Vertex AI Provider.
func NewVertexProvider(cfg VertexConfig) LLMProvider {
	loc := cfg.Location
	if loc == "" {
		loc = "us-central1"
	}
	model := cfg.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &vertexProvider{
		project:       cfg.Project,
		location:      loc,
		model:         model,
		bearerToken:   cfg.BearerToken,
		apiKey:        cfg.APIKey,
		customBaseURL: cfg.CustomBaseURL,
		httpClient:    client,
	}
}

func (p *vertexProvider) Name() string {
	return "vertex"
}

func (p *vertexProvider) Generate(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	toolReq := ToolCompletionRequest{
		CompletionRequest: req,
	}
	resp, err := p.GenerateWithTools(ctx, toolReq)
	if err != nil {
		return nil, err
	}
	return &CompletionResponse{
		Text:  resp.Text,
		Model: resp.Model,
		Usage: resp.Usage,
	}, nil
}

func (p *vertexProvider) GenerateWithTools(ctx context.Context, req ToolCompletionRequest) (*ToolCompletionResponse, error) {
	contents := make([]map[string]interface{}, 0, len(req.Messages))

	for _, m := range req.Messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}

		parts := make([]map[string]interface{}, 0)

		if m.Content != "" {
			parts = append(parts, map[string]interface{}{"text": m.Content})
		}

		for _, tc := range m.ToolCalls {
			var argsMap map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Arguments), &argsMap); err != nil {
				argsMap = map[string]interface{}{"raw_args": tc.Arguments}
			}
			parts = append(parts, map[string]interface{}{
				"functionCall": map[string]interface{}{
					"name": tc.Name,
					"args": argsMap,
				},
			})
		}

		if m.Role == "tool" {
			role = "user"
			parts = []map[string]interface{}{
				{
					"functionResponse": map[string]interface{}{
						"name": m.Name,
						"response": map[string]interface{}{
							"content": m.Content,
						},
					},
				},
			}
		}

		if len(parts) > 0 {
			contents = append(contents, map[string]interface{}{
				"role":  role,
				"parts": parts,
			})
		}
	}

	payload := map[string]interface{}{
		"contents": contents,
	}

	if req.SystemPrompt != "" {
		payload["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": req.SystemPrompt},
			},
		}
	}

	if len(req.Tools) > 0 {
		declarations := lo.Map(req.Tools, func(t ToolDefinition, _ int) map[string]interface{} {
			decl := map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
			}
			if len(t.Parameters) > 0 {
				decl["parameters"] = t.Parameters
			}
			return decl
		})

		payload["tools"] = []map[string]interface{}{
			{"functionDeclarations": declarations},
		}
	}

	reqBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal vertex request: %w", err)
	}

	endpoint := p.buildEndpoint()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("create vertex request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.bearerToken != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.bearerToken))
	}
	if p.apiKey != "" {
		httpReq.Header.Set("x-goog-api-key", p.apiKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute vertex request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read vertex response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vertex request failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var vertexResp struct {
		Candidates []struct {
			Content struct {
				Role  string `json:"role"`
				Parts []struct {
					Text         string `json:"text,omitempty"`
					FunctionCall *struct {
						Name string                 `json:"name"`
						Args map[string]interface{} `json:"args"`
					} `json:"functionCall,omitempty"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.Unmarshal(respBody, &vertexResp); err != nil {
		return nil, fmt.Errorf("unmarshal vertex response: %w", err)
	}

	if len(vertexResp.Candidates) == 0 {
		return nil, fmt.Errorf("no response candidates from vertex ai")
	}

	candidate := vertexResp.Candidates[0]
	var textParts []string
	var toolCalls []ToolCall

	for i, part := range candidate.Content.Parts {
		if part.Text != "" {
			textParts = append(textParts, part.Text)
		}
		if part.FunctionCall != nil {
			argsBytes, _ := json.Marshal(part.FunctionCall.Args)
			toolCalls = append(toolCalls, ToolCall{
				ID:        fmt.Sprintf("call_%d_%s", i, part.FunctionCall.Name),
				Name:      part.FunctionCall.Name,
				Arguments: string(argsBytes),
			})
		}
	}

	return &ToolCompletionResponse{
		Text:      strings.Join(textParts, "\n"),
		ToolCalls: toolCalls,
		Model:     p.model,
		Usage: TokenUsage{
			InputTokens:  vertexResp.UsageMetadata.PromptTokenCount,
			OutputTokens: vertexResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:  vertexResp.UsageMetadata.TotalTokenCount,
		},
	}, nil
}

func (p *vertexProvider) buildEndpoint() string {
	if p.customBaseURL != "" {
		return p.customBaseURL
	}
	if p.apiKey != "" && p.project == "" {
		return fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.model, p.apiKey)
	}
	if p.location == "global" || p.location == "" {
		return fmt.Sprintf("https://aiplatform.googleapis.com/v1/projects/%s/locations/global/publishers/google/models/%s:generateContent",
			p.project, p.model)
	}
	return fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		p.location, p.project, p.location, p.model)
}
