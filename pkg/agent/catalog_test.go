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

func TestParseGeminiVersion(t *testing.T) {
	tests := []struct {
		model    string
		wantMaj  int
		wantMin  int
		wantOk   bool
	}{
		{"gemini-3.8-flash", 3, 8, true},
		{"gemini-3.7-flash", 3, 7, true},
		{"gemini-3.5-flash", 3, 5, true},
		{"gemini-3.1-pro-preview", 3, 1, true},
		{"gemini-2.5-flash", 2, 5, true},
		{"gemini-1.5-pro", 1, 5, true},
		{"publishers/google/models/gemini-3.8-flash@default", 3, 8, true},
		{"claude-3-7-sonnet", 0, 0, false},
		{"gpt-4o", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			maj, min, ok := ParseGeminiVersion(tt.model)
			assert.Equal(t, tt.wantOk, ok)
			if ok {
				assert.Equal(t, tt.wantMaj, maj)
				assert.Equal(t, tt.wantMin, min)
			}
		})
	}
}

func TestIsGeminiVersionAllowed(t *testing.T) {
	// Must reject < 3.5, must allow >= 3.5
	allowed := []string{
		"gemini-3.5-flash",
		"gemini-3.6-flash",
		"gemini-3.7-flash",
		"gemini-3.8-flash",
		"gemini-3.9-flash",
		"gemini-4.0-flash",
		"gemini-3.5-pro",
		// Non-gemini models should not be restricted
		"claude-3-7-sonnet",
		"amazon.nova-pro-v1:0",
	}
	for _, m := range allowed {
		assert.True(t, IsGeminiVersionAllowed(m), "expected %s to be allowed", m)
	}

	disallowed := []string{
		"gemini-1.5-flash",
		"gemini-1.5-pro",
		"gemini-2.0-flash",
		"gemini-2.0-flash-lite",
		"gemini-2.5-flash",
		"gemini-2.5-pro",
		"gemini-3.1-flash",
		"gemini-3.1-pro",
	}
	for _, m := range disallowed {
		assert.False(t, IsGeminiVersionAllowed(m), "expected %s to be disallowed (< 3.5)", m)
	}
}

func TestEnforceGeminiMinimumVersion(t *testing.T) {
	// Should upgrade disallowed < 3.5 models to default latest (gemini-3.8-flash)
	assert.Equal(t, DefaultLatestGeminiFlashModel, EnforceGeminiMinimumVersion("gemini-2.5-flash"))
	assert.Equal(t, DefaultLatestGeminiFlashModel, EnforceGeminiMinimumVersion("gemini-1.5-flash"))
	assert.Equal(t, DefaultLatestGeminiFlashModel, EnforceGeminiMinimumVersion("gemini-3.1-flash"))

	// Should preserve >= 3.5 models
	assert.Equal(t, "gemini-3.5-flash", EnforceGeminiMinimumVersion("gemini-3.5-flash"))
	assert.Equal(t, "gemini-3.7-flash", EnforceGeminiMinimumVersion("gemini-3.7-flash"))
	assert.Equal(t, "gemini-3.8-flash", EnforceGeminiMinimumVersion("gemini-3.8-flash"))

	// Should preserve non-gemini models
	assert.Equal(t, "claude-sonnet", EnforceGeminiMinimumVersion("claude-sonnet"))
}

func TestSelectLatestGeminiFlashModel(t *testing.T) {
	mockCatalog := []string{
		"publishers/google/models/gemini-embedding-001",
		"publishers/google/models/gemini-2.5-pro",
		"publishers/google/models/gemini-2.5-flash",
		"publishers/google/models/gemini-3.5-flash@default",
		"publishers/google/models/gemini-3.7-flash",
		"publishers/google/models/gemini-3.6-flash",
		"publishers/google/models/gemini-3.8-flash@default",
		"publishers/google/models/gemini-3.1-flash-lite",
		"publishers/google/models/gemini-2.5-flash-lite",
	}

	selected := SelectLatestGeminiFlashModel(mockCatalog)
	assert.Equal(t, "gemini-3.8-flash", selected)

	// If future 3.9 or 4.0 appears
	futureCatalog := append(mockCatalog, "publishers/google/models/gemini-3.9-flash")
	assert.Equal(t, "gemini-3.9-flash", SelectLatestGeminiFlashModel(futureCatalog))

	futureMajorCatalog := append(mockCatalog, "publishers/google/models/gemini-4.1-flash")
	assert.Equal(t, "gemini-4.1-flash", SelectLatestGeminiFlashModel(futureMajorCatalog))

	// If only < 3.5 exists in list, fallback to default
	onlyOld := []string{"gemini-2.5-flash", "gemini-1.5-flash"}
	assert.Equal(t, DefaultLatestGeminiFlashModel, SelectLatestGeminiFlashModel(onlyOld))
}

func TestFetchGeminiCatalogModels_MockServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/publishers/google/models", r.URL.Path)
		assert.Equal(t, "Bearer mock-token", r.Header.Get("Authorization"))
		assert.Equal(t, "mock-project", r.Header.Get("X-Goog-User-Project"))

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"publisherModels": [
				{"name": "publishers/google/models/gemini-2.5-flash"},
				{"name": "publishers/google/models/gemini-3.7-flash"},
				{"name": "publishers/google/models/gemini-3.8-flash"}
			]
		}`)
	}))
	defer ts.Close()

	cfg := VertexConfig{
		Project:       "mock-project",
		Location:      "us-central1",
		BearerToken:   "mock-token",
		CustomBaseURL: ts.URL,
	}

	models, err := FetchGeminiCatalogModels(context.Background(), cfg)
	require.NoError(t, err)
	assert.Len(t, models, 3)

	selected := SelectLatestGeminiFlashModel(models)
	assert.Equal(t, "gemini-3.8-flash", selected)
}

func TestResolveLatestGeminiFlashModel(t *testing.T) {
	// 1. Explicit model >= 3.5 -> returned as-is
	cfg := VertexConfig{Model: "gemini-3.7-flash"}
	assert.Equal(t, "gemini-3.7-flash", ResolveLatestGeminiFlashModel(context.Background(), cfg))

	// 2. Explicit model < 3.5 -> upgraded to DefaultLatestGeminiFlashModel
	cfgOld := VertexConfig{Model: "gemini-2.5-flash"}
	assert.Equal(t, DefaultLatestGeminiFlashModel, ResolveLatestGeminiFlashModel(context.Background(), cfgOld))

	// 3. "auto" or "" with mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"publisherModels": [
				{"name": "publishers/google/models/gemini-3.5-flash"},
				{"name": "publishers/google/models/gemini-3.8-flash"}
			]
		}`)
	}))
	defer ts.Close()

	// Clear cache for test
	cachedModelMu.Lock()
	cachedResolvedModel = ""
	cachedModelExpiry = time.Time{}
	cachedModelMu.Unlock()

	cfgAuto := VertexConfig{
		Model:         "auto",
		CustomBaseURL: ts.URL,
	}
	resolved := ResolveLatestGeminiFlashModel(context.Background(), cfgAuto)
	assert.Equal(t, "gemini-3.8-flash", resolved)
}
