package bootstrap

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactSecret(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		want   string
	}{
		{"empty stays empty", "", ""},
		{"short fully masked", "abc", "********"},
		{"boundary length masked", "12345678", "********"},
		{"long keeps edges", "0123456789abcdef", "0123****cdef"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, redactSecret(tt.secret), tt.name)
	}
}

func TestMaskFlagDefaults(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("client-secret", "super-secret-value-123", "secret")
	cmd.Flags().String("client-id", "37a4559da8d5", "public")
	cmd.Flags().String("absent", "", "not bound here")

	maskFlagDefaults(cmd, "client-secret", "client-id", "absent", "missing-flag")

	assert.Equal(t, "supe****-123", cmd.Flags().Lookup("client-secret").DefValue)
	assert.Equal(t, "37a4****a8d5", cmd.Flags().Lookup("client-id").DefValue)
	// Runtime value is untouched; only the displayed default changes.
	assert.Equal(t, "super-secret-value-123", cmd.Flags().Lookup("client-secret").Value.String())
	assert.Equal(t, "", cmd.Flags().Lookup("absent").DefValue)
}

func TestServeCommand_HelpMasksClientSecret(t *testing.T) {
	t.Setenv("KAKAO_CLIENT_SECRET", "e2e-test-secret-value")
	cmd := NewRootCommand(context.Background())
	serve, _, err := cmd.Find([]string{"serve"})
	require.NoError(t, err)
	secretFlag := serve.Flags().Lookup("client-secret")
	require.NotNil(t, secretFlag)

	assert.Equal(t, "e2e-****alue", secretFlag.DefValue)
	assert.Equal(t, "e2e-test-secret-value", secretFlag.Value.String())
}
