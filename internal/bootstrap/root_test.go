package bootstrap

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommand(t *testing.T) {
	cmd := NewRootCommand(context.Background())
	assert.Equal(t, "kakaobot", cmd.Use)
	assert.True(t, cmd.SilenceUsage)
	assert.True(t, cmd.SilenceErrors)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "KakaoTalk CLI bot")
	assert.Contains(t, buf.String(), "auth")
	assert.Contains(t, buf.String(), "send")
	assert.Contains(t, buf.String(), "friends")
	assert.Contains(t, buf.String(), "user")
	assert.Contains(t, buf.String(), "storage")
	assert.Contains(t, buf.String(), "serve")
	assert.Contains(t, buf.String(), "skill")
}
