package bootstrap

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserCommand_Help(t *testing.T) {
	cmd := NewRootCommand(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"user", "--help"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Retrieve Kakao user profile")
}

func TestUserMeCommand_PreRunE(t *testing.T) {
	cmd := newUserMeCommand(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--client-id", "test"})

	err := cmd.PreRunE(cmd, []string{"--client-id", "test"})
	assert.NoError(t, err)
}
