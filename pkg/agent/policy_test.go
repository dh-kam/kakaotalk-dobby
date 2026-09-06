package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvePolicyDefaultModel(t *testing.T) {
	// Date before 2027-01-01 -> latest Gemini Flash (DefaultLatestGeminiFlashModel)
	beforeCutoff := time.Date(2026, time.December, 31, 23, 59, 59, 0, time.FixedZone("KST", 9*3600))
	cfg := VertexConfig{}
	assert.Equal(t, DefaultLatestGeminiFlashModel, ResolvePolicyDefaultModel(beforeCutoff, cfg))

	// Date on or after 2027-01-01 -> Grok non-reasoning
	atCutoff := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.FixedZone("KST", 9*3600))
	assert.Equal(t, ModelGrok420NonReasoning, ResolvePolicyDefaultModel(atCutoff, cfg))

	afterCutoff := time.Date(2027, time.February, 15, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	assert.Equal(t, ModelGrok420NonReasoning, ResolvePolicyDefaultModel(afterCutoff, cfg))
}

func TestModelPreferenceStore(t *testing.T) {
	store := &inMemoryModelPreferenceStore{
		preferences: make(map[string]string),
	}

	assert.Equal(t, "", store.Get("user1"))

	store.Set("user1", "grok-4.20-non-reasoning")
	assert.Equal(t, "grok-4.20-non-reasoning", store.Get("user1"))

	store.Set("user1", "auto")
	assert.Equal(t, "", store.Get("user1"))

	store.Set("user2", "gemini-3.8-flash")
	assert.Equal(t, "gemini-3.8-flash", store.Get("user2"))

	store.Reset("user2")
	assert.Equal(t, "", store.Get("user2"))
}

func TestNormalizeModelAlias(t *testing.T) {
	tests := []struct {
		input     string
		wantModel string
		wantOk    bool
	}{
		{"gemini", "gemini-3.8-flash", true},
		{"flash", "gemini-3.8-flash", true},
		{"3.8", "gemini-3.8-flash", true},
		{"gemini-3.8-flash", "gemini-3.8-flash", true},
		{"grok", "grok-4.20-non-reasoning", true},
		{"4.20", "grok-4.20-non-reasoning", true},
		{"grok-4.20-non-reasoning", "grok-4.20-non-reasoning", true},
		{"4.3", "grok-4.3", true},
		{"grok-4.3", "grok-4.3", true},
		{"auto", "auto", true},
		{"기본", "auto", true},
		{"reset", "auto", true},
		{"unknown-model", "", false},
	}

	for _, tt := range tests {
		got, ok := NormalizeModelAlias(tt.input)
		assert.Equal(t, tt.wantOk, ok, "input=%q", tt.input)
		if ok {
			assert.Equal(t, tt.wantModel, got, "input=%q", tt.input)
		}
	}
}

func TestDynamicLLMProvider(t *testing.T) {
	var requestedPaths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"candidates": [
				{
					"content": {
						"role": "model",
						"parts": [{"text": "Dynamic reply"}]
					}
				}
			]
		}`)
	}))
	defer ts.Close()

	store := &inMemoryModelPreferenceStore{
		preferences: make(map[string]string),
	}

	// 1. Before 2027-01-01 with no user preference -> policy picks gemini
	fixedTime2026 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	provider := NewDynamicLLMProvider(DynamicProviderConfig{
		VertexCfg: VertexConfig{
			APIKey:        "test-key",
			CustomBaseURL: ts.URL,
		},
		PrefStore: store,
		TimeFunc:  func() time.Time { return fixedTime2026 },
	})

	ctx := context.Background()
	resp, err := provider.Generate(ctx, CompletionRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Dynamic reply", resp.Text)
	require.NotEmpty(t, requestedPaths)
	assert.Contains(t, requestedPaths[len(requestedPaths)-1], "gemini-3.8-flash")

	// 2. User sets preference to grok-4.20-non-reasoning -> routes to grok with global location
	store.Set("user_abc", "grok-4.20-non-reasoning")
	ctxUser := WithUserID(context.Background(), "user_abc")

	respUser, err := provider.Generate(ctxUser, CompletionRequest{
		Messages: []Message{{Role: "user", Content: "hi grok"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Dynamic reply", respUser.Text)
	assert.Contains(t, requestedPaths[len(requestedPaths)-1], "publishers/xai/models/grok-4.20-non-reasoning")

	// 3. After 2027-01-01 with no preference -> policy picks grok-4.20-non-reasoning
	fixedTime2027 := time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC)
	provider2027 := NewDynamicLLMProvider(DynamicProviderConfig{
		VertexCfg: VertexConfig{
			APIKey:        "test-key",
			CustomBaseURL: ts.URL,
		},
		PrefStore: store,
		TimeFunc:  func() time.Time { return fixedTime2027 },
	})

	resp2027, err := provider2027.Generate(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "user", Content: "2027 hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Dynamic reply", resp2027.Text)
	assert.Contains(t, requestedPaths[len(requestedPaths)-1], "publishers/xai/models/grok-4.20-non-reasoning")
}
