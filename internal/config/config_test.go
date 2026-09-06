package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveAIKey(t *testing.T) {
	t.Run("explicit AI_API_KEY wins", func(t *testing.T) {
		t.Setenv("AI_API_KEY", "direct")
		t.Setenv("XAI_API_KEY", "xai-key")
		assert.Equal(t, "direct", resolveAIKey("grok"))
	})

	t.Run("grok provider pairs with XAI key even when vertex key exists", func(t *testing.T) {
		t.Setenv("VERTEX_API_KEY", "vertex-key")
		t.Setenv("XAI_API_KEY", "xai-key")
		assert.Equal(t, "xai-key", resolveAIKey("grok"))
	})

	t.Run("gemini provider prefers GEMINI then VERTEX", func(t *testing.T) {
		t.Setenv("VERTEX_API_KEY", "vertex-key")
		assert.Equal(t, "vertex-key", resolveAIKey("gemini"))

		t.Setenv("GEMINI_API_KEY", "gemini-key")
		assert.Equal(t, "gemini-key", resolveAIKey("gemini"))
	})

	t.Run("claude provider pairs with anthropic key", func(t *testing.T) {
		t.Setenv("VERTEX_API_KEY", "vertex-key")
		t.Setenv("ANTHROPIC_API_KEY", "anthropic-key")
		assert.Equal(t, "anthropic-key", resolveAIKey("claude"))
	})

	t.Run("unknown provider falls back to generic chain", func(t *testing.T) {
		t.Setenv("VERTEX_API_KEY", "vertex-key")
		t.Setenv("XAI_API_KEY", "xai-key")
		assert.Equal(t, "vertex-key", resolveAIKey(""))
		assert.Equal(t, "vertex-key", resolveAIKey("ollama"))
	})

	t.Run("no keys at all", func(t *testing.T) {
		assert.Equal(t, "", resolveAIKey("grok"))
	})
}
