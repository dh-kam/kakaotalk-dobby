package bootstrap

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStorageCommand_Help(t *testing.T) {
	cmd := NewRootCommand(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"storage", "--help"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Manage Talk Message image uploads")
}

func TestStorageUploadCommand_PreRunE_MissingFile(t *testing.T) {
	cmd := newStorageUploadCommand(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})

	err := cmd.PreRunE(cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image file path is required")
}
