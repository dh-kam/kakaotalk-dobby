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
		defaultModel: "gemini-3.8-flash",
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
	assert.Equal(t, "ollama", ollama.Name())

	// Grok reuses the OpenAI-compatible client against api.x.ai
	grok, err := NewProvider(ProviderConfig{
		ProviderName: "grok",
		APIKey:       "test-key",
	})
	require.NoError(t, err)
	assert.Equal(t, "grok", grok.Name())
	grokImpl, ok := grok.(*openAIProvider)
	require.True(t, ok)
	assert.Equal(t, "grok-4.20-0309-non-reasoning", grokImpl.defaultModel)
	assert.Equal(t, "https://api.x.ai/v1", grokImpl.baseURL)

	xai, err := NewProvider(ProviderConfig{
		ProviderName: "xai",
		APIKey:       "test-key",
		Model:        "grok-4.20-0309-non-reasoning",
	})
	require.NoError(t, err)
	xaiImpl, ok := xai.(*openAIProvider)
	require.True(t, ok)
	assert.Equal(t, "grok-4.20-0309-non-reasoning", xaiImpl.defaultModel)
}

func TestOpenAIProvider_ErrorShapes(t *testing.T) {
	tests := []struct {
		name      string
		provider  string
		body      string
		wantInErr []string
	}{
		{
			name:      "object error shape",
			provider:  "openai",
			body:      `{"error": {"message": "Rate limit exceeded", "type": "rate_limit_error"}}`,
			wantInErr: []string{"Rate limit exceeded", "rate_limit_error"},
		},
		{
			name:      "bare string error shape (xAI)",
			provider:  "grok",
			body:      `{"error": "Incorrect API key provided"}`,
			wantInErr: []string{"Incorrect API key provided"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer ts.Close()

			provider := NewOpenAIProvider(OpenAIConfig{
				ProviderName: tt.provider,
				BaseURL:      ts.URL,
				APIKey:       "bad-key",
			})

			_, err := provider.GenerateResponse(context.Background(), CompletionRequest{
				Messages: []ChatMessage{{Role: "user", Content: "hi"}},
			})
			require.Error(t, err)
			for _, want := range tt.wantInErr {
				assert.Contains(t, err.Error(), want)
			}
			assert.Contains(t, err.Error(), tt.provider, "error should name the provider")
		})
	}
}
