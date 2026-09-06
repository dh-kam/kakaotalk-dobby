package skill

import (
	"strings"

	"github.com/samber/lo"
)

// botName is the persona the fallback AI answers as.
const botName = "집 요정 도비"

// modelDisplayPrefixes maps model identifier prefixes to human-friendly names,
// longest prefix first so "deepseek" wins over any shorter overlapping entry.
var modelDisplayPrefixes = []struct{ prefix, display string }{
	{"deepseek", "DeepSeek"},
	{"gemini", "Gemini"},
	{"claude", "Claude"},
	{"grok", "Grok"},
	{"gpt", "GPT"},
	{"llama", "Llama"},
	{"mistral", "Mistral"},
	{"qwen", "Qwen"},
	{"mock", "Mock"},
}

// aiModelDisplayName resolves a raw model identifier to a display name such as
// "Grok" for "grok-4.20-0309-non-reasoning". Unknown or empty models resolve
// to "AI" so the identity line stays well-formed.
func aiModelDisplayName(model string) string {
	normalized := strings.ToLower(strings.TrimSpace(model))
	match, found := lo.Find(modelDisplayPrefixes, func(entry struct{ prefix, display string }) bool {
		return strings.HasPrefix(normalized, entry.prefix)
	})
	if !found {
		return "AI"
	}
	return match.display
}

// withBotIdentity prefixes an identity block so the fallback model introduces
// itself as the bot persona instead of its underlying base model.
func withBotIdentity(systemPrompt, modelDisplay string) string {
	identity := botName + "(feat. " + modelDisplay + ")"

	var b strings.Builder
	b.WriteString("너는 카카오톡 챗봇 \"")
	b.WriteString(identity)
	b.WriteString("\"이다.\n\n신원 규칙:\n")
	b.WriteString("- 너의 이름은 \"")
	b.WriteString(botName)
	b.WriteString("\"이며, 필요하면 \"")
	b.WriteString(identity)
	b.WriteString("\"라고 소개한다.\n")
	b.WriteString("- 어떤 질문을 받아도 스스로를 다른 이름이나 다른 AI 모델로 밝히지 않는다.\n")
	b.WriteString("- 편안한 존댓말로, 모바일 채팅에 맞게 짧고 명확하게 답한다.\n")

	if prompt := strings.TrimSpace(systemPrompt); prompt != "" {
		b.WriteString("\n")
		b.WriteString(prompt)
	}
	return b.String()
}
