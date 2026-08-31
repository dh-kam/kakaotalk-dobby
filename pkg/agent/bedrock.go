package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/samber/lo"
)

type bedrockProvider struct {
	region        string
	modelID       string
	bearerToken   string
	accessKeyID   string
	secretKey     string
	customBaseURL string
	httpClient    *http.Client
}

// BedrockConfig holds configuration for AWS Bedrock.
type BedrockConfig struct {
	Region        string
	ModelID       string
	BearerToken   string // Optional Bearer token or Bedrock proxy token
	AccessKeyID   string // AWS Access Key
	SecretKey     string // AWS Secret Access Key
	CustomBaseURL string // For testing or custom proxy endpoints
	HTTPClient    *http.Client
}

// NewBedrockProvider creates an AWS Bedrock Provider using the Bedrock Converse API.
func NewBedrockProvider(cfg BedrockConfig) LLMProvider {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	modelID := cfg.ModelID
	if modelID == "" {
		modelID = "us.anthropic.claude-3-5-sonnet-20241022-v2:0"
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &bedrockProvider{
		region:        region,
		modelID:       modelID,
		bearerToken:   cfg.BearerToken,
		accessKeyID:   cfg.AccessKeyID,
		secretKey:     cfg.SecretKey,
		customBaseURL: cfg.CustomBaseURL,
		httpClient:    client,
	}
}

func (p *bedrockProvider) Name() string {
	return "bedrock"
}

func (p *bedrockProvider) Generate(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
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

func (p *bedrockProvider) GenerateWithTools(ctx context.Context, req ToolCompletionRequest) (*ToolCompletionResponse, error) {
	messages := make([]map[string]interface{}, 0, len(req.Messages))

	for _, m := range req.Messages {
		role := m.Role
		if role == "model" {
			role = "assistant"
		}

		contentItems := make([]map[string]interface{}, 0)

		if m.Content != "" && m.Role != "tool" {
			contentItems = append(contentItems, map[string]interface{}{"text": m.Content})
		}

		for _, tc := range m.ToolCalls {
			var inputMap map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Arguments), &inputMap); err != nil {
				inputMap = map[string]interface{}{"raw_input": tc.Arguments}
			}
			contentItems = append(contentItems, map[string]interface{}{
				"toolUse": map[string]interface{}{
					"toolUseId": tc.ID,
					"name":      tc.Name,
					"input":     inputMap,
				},
			})
		}

		if m.Role == "tool" {
			role = "user"
			contentItems = append(contentItems, map[string]interface{}{
				"toolResult": map[string]interface{}{
					"toolUseId": m.ToolCallID,
					"content": []map[string]interface{}{
						{"text": m.Content},
					},
				},
			})
		}

		if len(contentItems) > 0 {
			messages = append(messages, map[string]interface{}{
				"role":    role,
				"content": contentItems,
			})
		}
	}

	payload := map[string]interface{}{
		"messages": messages,
	}

	if req.SystemPrompt != "" {
		payload["system"] = []map[string]interface{}{
			{"text": req.SystemPrompt},
		}
	}

	if len(req.Tools) > 0 {
		toolsList := lo.Map(req.Tools, func(t ToolDefinition, _ int) map[string]interface{} {
			return map[string]interface{}{
				"toolSpec": map[string]interface{}{
					"name":        t.Name,
					"description": t.Description,
					"inputSchema": map[string]interface{}{
						"json": t.Parameters,
					},
				},
			}
		})

		payload["toolConfig"] = map[string]interface{}{
			"tools": toolsList,
		}
	}

	reqBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal bedrock request: %w", err)
	}

	endpoint := p.buildEndpoint()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("create bedrock request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.bearerToken != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.bearerToken))
		if strings.HasPrefix(p.bearerToken, "ABSK") {
			httpReq.Header.Set("x-api-key", p.bearerToken)
		}
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute bedrock request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read bedrock response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bedrock request failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var bedrockResp struct {
		Output struct {
			Message struct {
				Role    string `json:"role"`
				Content []struct {
					Text    string `json:"text,omitempty"`
					ToolUse *struct {
						ToolUseID string                 `json:"toolUseId"`
						Name      string                 `json:"name"`
						Input     map[string]interface{} `json:"input"`
					} `json:"toolUse,omitempty"`
				} `json:"content"`
			} `json:"message"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
			TotalTokens  int `json:"totalTokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &bedrockResp); err != nil {
		return nil, fmt.Errorf("unmarshal bedrock response: %w", err)
	}

	var textParts []string
	var toolCalls []ToolCall

	for _, item := range bedrockResp.Output.Message.Content {
		if item.Text != "" {
			textParts = append(textParts, item.Text)
		}
		if item.ToolUse != nil {
			argsBytes, _ := json.Marshal(item.ToolUse.Input)
			toolCalls = append(toolCalls, ToolCall{
				ID:        item.ToolUse.ToolUseID,
				Name:      item.ToolUse.Name,
				Arguments: string(argsBytes),
			})
		}
	}

	return &ToolCompletionResponse{
		Text:      strings.Join(textParts, "\n"),
		ToolCalls: toolCalls,
		Model:     p.modelID,
		Usage: TokenUsage{
			InputTokens:  bedrockResp.Usage.InputTokens,
			OutputTokens: bedrockResp.Usage.OutputTokens,
			TotalTokens:  bedrockResp.Usage.TotalTokens,
		},
	}, nil
}

func (p *bedrockProvider) buildEndpoint() string {
	if p.customBaseURL != "" {
		return p.customBaseURL
	}
	return fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/converse",
		p.region, url.PathEscape(p.modelID))
}
