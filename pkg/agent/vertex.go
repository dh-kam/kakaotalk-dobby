package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/lo"
	"google.golang.org/genai"
)

type vertexSDKProvider struct {
	project       string
	location      string
	model         string
	apiKey        string
	bearerToken   string
	customBaseURL string
	httpClient    *http.Client
	client        *genai.Client
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

// NewVertexProvider creates a new Google Vertex AI Provider using official google.golang.org/genai SDK.
func NewVertexProvider(cfg VertexConfig) LLMProvider {
	project := cfg.Project
	if project == "" {
		project = "c0de1ab-dev-494714"
	}
	loc := cfg.Location
	if loc == "" {
		loc = "global"
	}
	model := cfg.Model
	if model == "" {
		model = "gemini-3.7-flash"
	}

	return &vertexSDKProvider{
		project:       project,
		location:      loc,
		model:         model,
		apiKey:        cfg.APIKey,
		bearerToken:   cfg.BearerToken,
		customBaseURL: cfg.CustomBaseURL,
		httpClient:    cfg.HTTPClient,
	}
}

func (p *vertexSDKProvider) Name() string {
	return "vertex"
}

func (p *vertexSDKProvider) getClient(ctx context.Context) (*genai.Client, error) {
	if p.client != nil {
		return p.client, nil
	}

	backend := genai.BackendVertexAI
	if p.project == "" {
		backend = genai.BackendGeminiAPI
	}

	clientCfg := &genai.ClientConfig{
		APIKey:     p.apiKey,
		Project:    p.project,
		Location:   p.location,
		Backend:    backend,
		HTTPClient: p.httpClient,
	}
	if p.customBaseURL != "" {
		clientCfg.HTTPOptions.BaseURL = p.customBaseURL
	}

	client, err := genai.NewClient(ctx, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("init google genai client: %w", err)
	}
	p.client = client
	return p.client, nil
}

func (p *vertexSDKProvider) Generate(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
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

func (p *vertexSDKProvider) GenerateWithTools(ctx context.Context, req ToolCompletionRequest) (*ToolCompletionResponse, error) {
	client, err := p.getClient(ctx)
	if err != nil {
		return nil, err
	}

	var contents []*genai.Content

	for _, m := range req.Messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}

		var parts []*genai.Part

		if m.Content != "" && m.Role != "tool" {
			parts = append(parts, genai.NewPartFromText(m.Content))
		}

		for _, tc := range m.ToolCalls {
			var argsMap map[string]any
			if err := json.Unmarshal([]byte(tc.Arguments), &argsMap); err != nil {
				argsMap = map[string]any{"raw_args": tc.Arguments}
			}
			p := genai.NewPartFromFunctionCall(tc.Name, argsMap)
			if tc.ThoughtSignature != "" {
				p.ThoughtSignature = []byte(tc.ThoughtSignature)
			}
			parts = append(parts, p)
		}

		if m.Role == "tool" {
			role = "user"
			var respMap map[string]any
			if err := json.Unmarshal([]byte(m.Content), &respMap); err != nil {
				respMap = map[string]any{"output": m.Content}
			}
			parts = append(parts, genai.NewPartFromFunctionResponse(m.Name, respMap))
		}

		if len(parts) > 0 {
			contents = append(contents, &genai.Content{
				Role:  role,
				Parts: parts,
			})
		}
	}

	genConfig := &genai.GenerateContentConfig{
		ThinkingConfig: &genai.ThinkingConfig{
			ThinkingBudget: lo.ToPtr(int32(0)),
		},
	}

	if req.SystemPrompt != "" {
		genConfig.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{genai.NewPartFromText(req.SystemPrompt)},
		}
	}

	if len(req.Tools) > 0 {
		var decls []*genai.FunctionDeclaration
		for _, t := range req.Tools {
			decl := &genai.FunctionDeclaration{
				Name:                 t.Name,
				Description:          t.Description,
				ParametersJsonSchema: t.Parameters,
			}
			decls = append(decls, decl)
		}
		genConfig.Tools = []*genai.Tool{
			{FunctionDeclarations: decls},
		}
	}

	resp, err := client.Models.GenerateContent(ctx, p.model, contents, genConfig)
	if err != nil {
		return nil, fmt.Errorf("vertex genai generate: %w", err)
	}

	var textParts []string
	var toolCalls []ToolCall

	for _, cand := range resp.Candidates {
		if cand.Content == nil {
			continue
		}
		for i, part := range cand.Content.Parts {
			if part.Text != "" {
				textParts = append(textParts, part.Text)
			}
			if part.FunctionCall != nil {
				argsBytes, _ := json.Marshal(part.FunctionCall.Args)
				toolCalls = append(toolCalls, ToolCall{
					ID:               fmt.Sprintf("call_%d_%s", i, part.FunctionCall.Name),
					Name:             part.FunctionCall.Name,
					Arguments:        string(argsBytes),
					ThoughtSignature: string(part.ThoughtSignature),
				})
			}
		}
	}

	var inputTokens, outputTokens, totalTokens int
	if resp.UsageMetadata != nil {
		inputTokens = int(resp.UsageMetadata.PromptTokenCount)
		outputTokens = int(resp.UsageMetadata.CandidatesTokenCount)
		totalTokens = int(resp.UsageMetadata.TotalTokenCount)
	}

	return &ToolCompletionResponse{
		Text:      strings.Join(textParts, "\n"),
		ToolCalls: toolCalls,
		Model:     p.model,
		Usage: TokenUsage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  totalTokens,
		},
	}, nil
}
