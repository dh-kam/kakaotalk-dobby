package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildDynamicSystemPrompt(t *testing.T) {
	base := "You are a bot."
	prompt := BuildDynamicSystemPrompt(base)

	assert.Contains(t, prompt, "You are a bot.")
	assert.Contains(t, prompt, "Live Korean Standard Time (KST) & Calendar Context")
	assert.Contains(t, prompt, "Current Time:")
	assert.Contains(t, prompt, "Today's Date:")
	assert.Contains(t, prompt, "Calendar Status:")
	assert.Contains(t, prompt, "Time & Context Reasoning Instructions")
	assert.Contains(t, prompt, "Default Time Context:")
	assert.Contains(t, prompt, "Relative Time Calculations:")
}
