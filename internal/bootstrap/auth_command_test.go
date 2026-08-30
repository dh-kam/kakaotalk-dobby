package bootstrap

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthCommand_Help(t *testing.T) {
	cmd := NewRootCommand(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"auth", "--help"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Manage KakaoTalk OAuth")
}

func TestAuthLoginCommand_PreRunE_MissingClientID(t *testing.T) {
	t.Setenv("KAKAO_REST_API_KEY", "")
	cmd := NewRootCommand(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"auth", "login"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--client-id is required")
}

func TestAuthRefreshCommand_PreRunE_MissingClientID(t *testing.T) {
	t.Setenv("KAKAO_REST_API_KEY", "")
	cmd := NewRootCommand(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"auth", "refresh"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--client-id is required")
}
