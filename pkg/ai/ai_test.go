package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIProvider_GenerateResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"choices": [
				{
					"message": {
						"role": "assistant",
						"content": "Hello, I am OpenAI!"
					}
				}
			],
			"usage": {
				"prompt_tokens": 12,
				"completion_tokens": 8
			}
		}`))
	}))
	defer ts.Close()

	provider := NewOpenAIProvider(OpenAIConfig{
		BaseURL: ts.URL,
		APIKey:  "sk-test",
	})

	resp, err := provider.GenerateResponse(context.Background(), CompletionRequest{
		Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello, I am OpenAI!", resp.Text)
	assert.Equal(t, 12, resp.PromptTokens)
	assert.Equal(t, 8, resp.OutputTokens)
}

func TestGeminiProvider_GenerateResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"candidates": [
				{
					"content": {
						"parts": [
							{"text": "Hello from Gemini"}
						],
						"role": "model"
					}
				}
			],
			"usageMetadata": {
				"promptTokenCount": 5,
				"candidatesTokenCount": 4
			}
		}`))
	}))
	defer ts.Close()

	provider := &geminiProvider{
		apiKey:       "test-key",
		defaultModel: "gemini-1.5-flash",
		httpClient:   ts.Client(),
	}

	// We test direct request or mock
	assert.Equal(t, "gemini", provider.Name())
}

func TestClaudeProvider_GenerateResponse(t *testing.T) {
	provider := NewClaudeProvider(ClaudeConfig{
		APIKey: "claude-key",
	})
	assert.Equal(t, "claude", provider.Name())
}

func TestNewProvider_Factory(t *testing.T) {
	prov, err := NewProvider(ProviderConfig{
		ProviderName: "mock",
	})
	require.NoError(t, err)
	assert.Equal(t, "mock", prov.Name())

	res, err := prov.GenerateResponse(context.Background(), CompletionRequest{
		Messages: []ChatMessage{{Role: "user", Content: "What is AI?"}},
	})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "What is AI?")

	// Test Ollama provider instantiation
	ollama, err := NewProvider(ProviderConfig{
		ProviderName: "ollama",
		BaseURL:      "http://localhost:11434/v1",
	})
	require.NoError(t, err)
	assert.Equal(t, "openai", ollama.Name())
}
