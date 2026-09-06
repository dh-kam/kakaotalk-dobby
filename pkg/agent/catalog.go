package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultLatestGeminiFlashModel is the default latest verified Gemini Flash model (>= 3.5).
	DefaultLatestGeminiFlashModel = "gemini-3.8-flash"

	// MinGeminiMajorVersion is the minimum supported Gemini major version.
	MinGeminiMajorVersion = 3

	// MinGeminiMinorVersion is the minimum supported Gemini minor version when major == 3.
	MinGeminiMinorVersion = 5
)

var (
	// flashModelRegex matches standard Gemini flash models like "gemini-3.8-flash" or "gemini-3.7-flash".
	flashModelRegex = regexp.MustCompile(`gemini-(\d+)\.(\d+)-flash$`)
	// generalGeminiRegex matches any gemini model with version like "gemini-2.5-pro", "gemini-1.5-flash".
	generalGeminiRegex = regexp.MustCompile(`gemini-(\d+)\.(\d+)`)

	cachedResolvedModel string
	cachedModelMu       sync.RWMutex
	cachedModelExpiry   time.Time
)

// GeminiVersion represents a parsed version from a Gemini model name.
type GeminiVersion struct {
	Major int
	Minor int
	Model string
}

// ParseGeminiVersion parses major and minor version numbers from a model name.
func ParseGeminiVersion(model string) (int, int, bool) {
	m := generalGeminiRegex.FindStringSubmatch(model)
	if len(m) < 3 {
		return 0, 0, false
	}
	major, err1 := strconv.Atoi(m[1])
	minor, err2 := strconv.Atoi(m[2])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return major, minor, true
}

// IsGeminiVersionAllowed returns true if the model is not a Gemini model,
// or if it is a Gemini model with version >= 3.5.
// Any Gemini version < 3.5 (e.g. 1.5, 2.0, 2.5, 3.1) returns false.
func IsGeminiVersionAllowed(model string) bool {
	if !strings.Contains(strings.ToLower(model), "gemini") {
		return true
	}
	major, minor, ok := ParseGeminiVersion(model)
	if !ok {
		// If version cannot be parsed, allow if it's explicitly set or not clearly < 3.5
		return true
	}
	if major < MinGeminiMajorVersion {
		return false
	}
	if major == MinGeminiMajorVersion && minor < MinGeminiMinorVersion {
		return false
	}
	return true
}

// EnforceGeminiMinimumVersion checks if the specified model satisfies Gemini >= 3.5.
// If it is a Gemini model < 3.5, it upgrades and returns the default latest flash model.
func EnforceGeminiMinimumVersion(model string) string {
	if model == "" || model == "auto" || model == "latest" {
		return DefaultLatestGeminiFlashModel
	}
	if !IsGeminiVersionAllowed(model) {
		return DefaultLatestGeminiFlashModel
	}
	return model
}

// SelectLatestGeminiFlashModel filters and selects the highest versioned Flash model >= 3.5 from a model list.
func SelectLatestGeminiFlashModel(models []string) string {
	var candidates []GeminiVersion

	for _, name := range models {
		// Strip publisher prefix if present (e.g. "publishers/google/models/gemini-3.8-flash" -> "gemini-3.8-flash")
		parts := strings.Split(name, "/")
		baseName := parts[len(parts)-1]
		// Strip version suffix if present (e.g. "gemini-3.8-flash@default" -> "gemini-3.8-flash")
		baseName = strings.Split(baseName, "@")[0]

		m := flashModelRegex.FindStringSubmatch(baseName)
		if len(m) < 3 {
			continue
		}
		major, err1 := strconv.Atoi(m[1])
		minor, err2 := strconv.Atoi(m[2])
		if err1 != nil || err2 != nil {
			continue
		}

		// Filter < 3.5
		if major < MinGeminiMajorVersion || (major == MinGeminiMajorVersion && minor < MinGeminiMinorVersion) {
			continue
		}

		candidates = append(candidates, GeminiVersion{
			Major: major,
			Minor: minor,
			Model: baseName,
		})
	}

	if len(candidates) == 0 {
		return DefaultLatestGeminiFlashModel
	}

	// Sort descending: highest major, then highest minor
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Major != candidates[j].Major {
			return candidates[i].Major > candidates[j].Major
		}
		return candidates[i].Minor > candidates[j].Minor
	})

	return candidates[0].Model
}

type publisherModelListResponse struct {
	PublisherModels []struct {
		Name string `json:"name"`
	} `json:"publisherModels"`
}

// FetchGeminiCatalogModels queries the Vertex AI Model Garden REST API for available Google publisher models.
func FetchGeminiCatalogModels(ctx context.Context, cfg VertexConfig) ([]string, error) {
	loc := cfg.Location
	if loc == "" || loc == "global" {
		loc = "us-central1"
	}

	baseURL := cfg.CustomBaseURL
	var reqURL string
	if baseURL != "" {
		reqURL = strings.TrimRight(baseURL, "/") + "/publishers/google/models?alt=json"
	} else {
		reqURL = fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1beta1/publishers/google/models?alt=json", loc)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create catalog request: %w", err)
	}

	if cfg.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.BearerToken)
	}
	if cfg.Project != "" {
		req.Header.Set("X-Goog-User-Project", cfg.Project)
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("catalog query status %d: %s", resp.StatusCode, string(body))
	}

	var data publisherModelListResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode catalog response: %w", err)
	}

	var models []string
	for _, m := range data.PublisherModels {
		if strings.Contains(m.Name, "gemini") {
			models = append(models, m.Name)
		}
	}

	return models, nil
}

// ResolveLatestGeminiFlashModel resolves the latest Gemini Flash model >= 3.5.
// If an explicit model is provided and satisfies >= 3.5, it is used.
// If model is empty, "auto", or "latest", it queries the catalog or uses the latest verified model.
func ResolveLatestGeminiFlashModel(ctx context.Context, cfg VertexConfig) string {
	model := strings.TrimSpace(cfg.Model)

	// If explicit model specified
	if model != "" && model != "auto" && model != "latest" {
		return EnforceGeminiMinimumVersion(model)
	}

	// Fast path for Vertex Express Mode (APIKey only): Express Mode does not expose publisher catalog
	if cfg.APIKey != "" && cfg.BearerToken == "" && cfg.CustomBaseURL == "" {
		return DefaultLatestGeminiFlashModel
	}

	// Check cache
	cachedModelMu.RLock()
	if cachedResolvedModel != "" && time.Now().Before(cachedModelExpiry) {
		cached := cachedResolvedModel
		cachedModelMu.RUnlock()
		return cached
	}
	cachedModelMu.RUnlock()

	// Try querying catalog
	models, err := FetchGeminiCatalogModels(ctx, cfg)
	if err == nil && len(models) > 0 {
		latest := SelectLatestGeminiFlashModel(models)
		if latest != "" {
			cachedModelMu.Lock()
			cachedResolvedModel = latest
			cachedModelExpiry = time.Now().Add(1 * time.Hour)
			cachedModelMu.Unlock()
			return latest
		}
	}

	// Negative cache fallback to verified latest default (15-minute backoff to protect SLA)
	cachedModelMu.Lock()
	cachedResolvedModel = DefaultLatestGeminiFlashModel
	cachedModelExpiry = time.Now().Add(15 * time.Minute)
	cachedModelMu.Unlock()

	return DefaultLatestGeminiFlashModel
}
