package agent

import (
	"context"
	"time"
)

var (
	// PolicyCutoffDate is 2027-01-01 00:00:00 KST (end of 2026-12-31).
	// Through 2026-12-31: Dynamic latest Gemini Flash model (e.g. gemini-3.8-flash).
	// From 2027-01-01: Grok non-reasoning model (grok-4.20-non-reasoning).
	PolicyCutoffDate = time.Date(2027, time.January, 1, 0, 0, 0, 0, time.FixedZone("KST", 9*3600))
)

const (
	// ModelGrok420NonReasoning is the cost-efficient standard non-reasoning Grok model.
	ModelGrok420NonReasoning = "grok-4.20-non-reasoning"
	// ModelGrok43 is the flagship reasoning and coding Grok model.
	ModelGrok43 = "grok-4.3"
)

// ResolvePolicyDefaultModel returns the policy-defined default model for a given timestamp.
// - Through December 31, 2026: dynamically chooses the latest Gemini Flash model from the catalog (>= 3.5).
// - From January 1, 2027 onwards: chooses grok-4.20-non-reasoning.
func ResolvePolicyDefaultModel(now time.Time, catalogCfg VertexConfig) string {
	if now.Before(PolicyCutoffDate) {
		return ResolveLatestGeminiFlashModel(context.Background(), catalogCfg)
	}
	return ModelGrok420NonReasoning
}
