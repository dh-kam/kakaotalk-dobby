package agent

import (
	"context"
	"strings"
	"sync"
	"time"
)

// DynamicProviderConfig configures a dynamic multi-model LLM provider.
type DynamicProviderConfig struct {
	VertexCfg VertexConfig
	PrefStore ModelPreferenceStore
	TimeFunc  func() time.Time
}

type dynamicLLMProvider struct {
	vertexCfg VertexConfig
	prefStore ModelPreferenceStore
	timeFunc  func() time.Time
	mu        sync.RWMutex
	providers map[string]LLMProvider
}

// NewDynamicLLMProvider creates an LLMProvider that dynamically selects the model
// based on user preferences and date policy (Gemini <= 2026-12-31, Grok >= 2027-01-01).
func NewDynamicLLMProvider(cfg DynamicProviderConfig) LLMProvider {
	store := cfg.PrefStore
	if store == nil {
		store = GetDefaultModelPreferenceStore()
	}
	tFunc := cfg.TimeFunc
	if tFunc == nil {
		tFunc = time.Now
	}

	return &dynamicLLMProvider{
		vertexCfg: cfg.VertexCfg,
		prefStore: store,
		timeFunc:  tFunc,
		providers: make(map[string]LLMProvider),
	}
}

func (p *dynamicLLMProvider) Name() string {
	return "dynamic"
}

func (p *dynamicLLMProvider) resolveModelForContext(ctx context.Context) string {
	userID := UserIDFromContext(ctx)
	if userID != "" {
		if preferred := p.prefStore.Get(userID); preferred != "" {
			return preferred
		}
	}
	return ResolvePolicyDefaultModel(p.timeFunc(), p.vertexCfg)
}

func (p *dynamicLLMProvider) getProviderForModel(model string) LLMProvider {
	p.mu.RLock()
	pr, ok := p.providers[model]
	p.mu.RUnlock()
	if ok {
		return pr
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if pr, ok := p.providers[model]; ok {
		return pr
	}

	// Create new provider for this model
	cfgCopy := p.vertexCfg
	cfgCopy.Model = model

	// If it's a Grok / xAI model, location MUST be global
	if strings.Contains(strings.ToLower(model), "grok") || strings.Contains(strings.ToLower(model), "xai") {
		cfgCopy.Location = "global"
		if !strings.HasPrefix(model, "publishers/") && !strings.HasPrefix(model, "projects/") {
			cfgCopy.Model = "publishers/xai/models/" + model
		}
	} else {
		if cfgCopy.Location == "" || cfgCopy.Location == "global" {
			cfgCopy.Location = "us-central1"
		}
	}

	newPr := NewVertexProvider(cfgCopy)
	p.providers[model] = newPr
	return newPr
}

func (p *dynamicLLMProvider) Generate(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := p.resolveModelForContext(ctx)
	return p.getProviderForModel(model).Generate(ctx, req)
}

func (p *dynamicLLMProvider) GenerateWithTools(ctx context.Context, req ToolCompletionRequest) (*ToolCompletionResponse, error) {
	model := p.resolveModelForContext(ctx)
	return p.getProviderForModel(model).GenerateWithTools(ctx, req)
}
