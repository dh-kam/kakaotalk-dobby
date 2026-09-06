package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestMultiBotConfig(t *testing.T) {
	t.Run("single bot backward compatibility", func(t *testing.T) {
		t.Setenv("KAKAO_BOTS", "")
		t.Setenv("KAKAO_REST_API_KEY", "legacy-key")
		t.Setenv("KAKAO_CLIENT_SECRET", "legacy-secret")

		cfg := Load()
		require.NotNil(t, cfg)
		assert.Equal(t, "legacy-key", cfg.ClientID)

		botCfg, ok := cfg.GetBotConfig("dobby")
		assert.True(t, ok)
		assert.Equal(t, "legacy-key", botCfg.ClientID)
		assert.True(t, botCfg.IsDefault)

		reg, err := cfg.BuildKakaoRegistry()
		require.NoError(t, err)
		assert.Equal(t, 1, reg.Count())
	})

	t.Run("multiple bots with explicit credentials", func(t *testing.T) {
		t.Setenv("KAKAO_BOTS", "dobby,infra")
		t.Setenv("KAKAO_BOT_DOBBY_REST_API_KEY", "dobby-key")
		t.Setenv("KAKAO_BOT_INFRA_REST_API_KEY", "infra-key")
		t.Setenv("KAKAO_BOT_INFRA_CLIENT_SECRET", "infra-sec")
		t.Setenv("KAKAO_DEFAULT_BOT", "infra")

		cfg := Load()
		require.NotNil(t, cfg)
		assert.Equal(t, "infra", cfg.DefaultBot)

		dobbyCfg, ok := cfg.GetBotConfig("dobby")
		assert.True(t, ok)
		assert.Equal(t, "dobby-key", dobbyCfg.ClientID)
		assert.False(t, dobbyCfg.IsDefault)

		infraCfg, ok := cfg.GetBotConfig("infra")
		assert.True(t, ok)
		assert.Equal(t, "infra-key", infraCfg.ClientID)
		assert.Equal(t, "infra-sec", infraCfg.ClientSecret)
		assert.True(t, infraCfg.IsDefault)

		reg, err := cfg.BuildKakaoRegistry()
		require.NoError(t, err)
		assert.Equal(t, 2, reg.Count())

		defaultClient, err := reg.GetDefault()
		require.NoError(t, err)
		assert.NotNil(t, defaultClient)
	})
}
