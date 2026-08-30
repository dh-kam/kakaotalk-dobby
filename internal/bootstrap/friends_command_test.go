package bootstrap

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFriendsCommand_Help(t *testing.T) {
	cmd := NewRootCommand(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"friends", "--help"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Inspect KakaoTalk friends")
}

func TestFriendsListCommand_PreRunE(t *testing.T) {
	cmd := newFriendsListCommand(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--limit", "5"})

	err := cmd.PreRunE(cmd, []string{"--limit", "5"})
	assert.NoError(t, err)
}
