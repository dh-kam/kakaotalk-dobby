package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVertexProvider_GenerateWithTools(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-api-key", r.Header.Get("x-goog-api-key"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"candidates": [
				{
					"content": {
						"role": "model",
						"parts": [
							{ "text": "Let me check the server status." },
							{ "functionCall": { "name": "get_server_status", "args": {} } }
						]
					},
					"finishReason": "STOP"
				}
			],
			"usageMetadata": {
				"promptTokenCount": 25,
				"candidatesTokenCount": 18,
				"totalTokenCount": 43
			}
		}`))
	}))
	defer ts.Close()

	provider := NewVertexProvider(VertexConfig{
		Project:       "my-gcp-project",
		Location:      "us-central1",
		Model:         "gemini-3.7-flash",
		APIKey:        "test-api-key",
		CustomBaseURL: ts.URL,
	})

	assert.Equal(t, "vertex", provider.Name())

	resp, err := provider.GenerateWithTools(context.Background(), ToolCompletionRequest{
		CompletionRequest: CompletionRequest{
			Messages: []Message{{Role: "user", Content: "How is the server?"}},
		},
		Tools: []ToolDefinition{
			{Name: "get_server_status", Description: "Get status"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "Let me check the server status.", resp.Text)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "get_server_status", resp.ToolCalls[0].Name)
	assert.Equal(t, 25, resp.Usage.InputTokens)
}

func TestBedrockProvider_GenerateWithTools(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer bedrock-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"output": {
				"message": {
					"role": "assistant",
					"content": [
						{ "text": "I will check the status on Bedrock." },
						{ "toolUse": { "toolUseId": "tool_123", "name": "get_server_status", "input": {} } }
					]
				}
			},
			"usage": {
				"inputTokens": 30,
				"outputTokens": 20,
				"totalTokens": 50
			}
		}`))
	}))
	defer ts.Close()

	provider := NewBedrockProvider(BedrockConfig{
		Region:        "us-east-1",
		ModelID:       "anthropic.claude-3-5-sonnet-20240620-v1:0",
		BearerToken:   "bedrock-token",
		CustomBaseURL: ts.URL,
	})

	assert.Equal(t, "bedrock", provider.Name())

	resp, err := provider.GenerateWithTools(context.Background(), ToolCompletionRequest{
		CompletionRequest: CompletionRequest{
			Messages: []Message{{Role: "user", Content: "Check status"}},
		},
		Tools: []ToolDefinition{
			{Name: "get_server_status", Description: "Get status"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "I will check the status on Bedrock.", resp.Text)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "get_server_status", resp.ToolCalls[0].Name)
	assert.Equal(t, "tool_123", resp.ToolCalls[0].ID)
}

func TestAgent_ReActLoopWithTools(t *testing.T) {
	mockProvider := NewMockLLMProvider("mock-v1")
	mockProvider.ToolCallPlan = []ToolCall{
		{
			ID:        "call_1",
			Name:      "get_server_status",
			Arguments: "{}",
		},
	}
	mockProvider.CustomAnswer = "The server is currently running stably on linux/amd64."

	tools := NewToolRegistry()
	tools.Register(&ServerStatusTool{})

	agent := NewAgent(AgentConfig{
		Provider:      mockProvider,
		Tools:         tools,
		SystemPrompt:  "You are a test agent.",
		MaxIterations: 3,
	})

	res, err := agent.Run(context.Background(), "Check server condition")
	require.NoError(t, err)
	assert.Contains(t, res.Output, "The server is currently running stably")
	require.Len(t, res.Steps, 2)
	assert.Equal(t, "get_server_status", res.Steps[0].ToolCalls[0].Name)
	assert.Contains(t, res.Steps[0].ToolResults[0].Output, "alloc_mb")
}

func TestNewLLMProvider_Factory(t *testing.T) {
	v, err := NewLLMProvider(ProviderOptions{
		ProviderName: "vertex",
		Project:      "proj",
		Location:     "asia-northeast3",
		Model:        "gemini-1.5-pro",
	})
	require.NoError(t, err)
	assert.Equal(t, "vertex", v.Name())

	b, err := NewLLMProvider(ProviderOptions{
		ProviderName: "bedrock",
		Region:       "ap-northeast-2",
		Model:        "anthropic.claude-3-5-sonnet",
	})
	require.NoError(t, err)
	assert.Equal(t, "bedrock", b.Name())
}
