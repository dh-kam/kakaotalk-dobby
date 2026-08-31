package bootstrap

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAgentCommand_Help(t *testing.T) {
	cmd := NewRootCommand(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"agent", "--help"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Run autonomous AI Agent")
}

func TestAgentRunCommand_PreRunE_MissingPrompt(t *testing.T) {
	cmd := newAgentRunCommand(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})

	err := cmd.PreRunE(cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prompt is required")
}
