package skill

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAIModelDisplayName(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{"grok-4.20-0309-non-reasoning", "Grok"},
		{"Grok-4.6", "Grok"},
		{"gpt-4o-mini", "GPT"},
		{"gemini-2.5-flash", "Gemini"},
		{"claude-sonnet-4-5", "Claude"},
		{"deepseek-chat", "DeepSeek"},
		{"llama-3.1-8b-instant", "Llama"},
		{"mock-model", "Mock"},
		{"", "AI"},
		{"   ", "AI"},
		{"some-unknown-model", "AI"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, aiModelDisplayName(tt.model), "model=%q", tt.model)
	}
}

func TestWithBotIdentity(t *testing.T) {
	got := withBotIdentity("너는 학습 도우미야.", "Grok")

	assert.True(t, strings.HasPrefix(got, "너는 카카오톡 챗봇 \"집 요정 도비(feat. Grok)\"이다."))
	assert.Contains(t, got, "집 요정 도비(feat. Grok)")
	assert.Contains(t, got, "스스로를 다른 이름이나 다른 AI 모델로 밝히지 않는다")
	assert.Contains(t, got, "너는 학습 도우미야.")
	assert.Less(t, strings.Index(got, "신원 규칙"), strings.Index(got, "너는 학습 도우미야."),
		"identity block must precede the original system prompt")
}

func TestWithBotIdentityEmptyPrompt(t *testing.T) {
	got := withBotIdentity("  ", "Mock")

	assert.Contains(t, got, "집 요정 도비(feat. Mock)")
	assert.NotContains(t, got, "\n\n\n")
}
