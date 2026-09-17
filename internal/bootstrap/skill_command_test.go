package bootstrap

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillServeValidatesAuthenticationBeforeStartup(t *testing.T) {
	t.Setenv("KAKAO_SKILL_TOKEN", "")
	cmd := newSkillServeCommand(context.Background())
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--skill-token="})
	err := cmd.Execute()
	require.ErrorContains(t, err, "KAKAO_SKILL_TOKEN is required")
	assert.Contains(t, output.String(), "Usage:")
	assert.NotContains(t, output.String(), "running on")
}

func TestSkillServeBindsAndMasksSkillToken(t *testing.T) {
	t.Setenv("KAKAO_SKILL_TOKEN", "test-skill-secret-from-env")
	cmd := newSkillServeCommand(context.Background())
	flag := cmd.Flags().Lookup("skill-token")
	require.NotNil(t, flag)
	assert.Equal(t, "test-skill-secret-from-env", flag.Value.String())
	assert.NotContains(t, flag.DefValue, "test-skill-secret-from-env")
	// Execute only validation; a valid secret must reach the normal RunE path.
	ran := false
	cmd.RunE = func(_ *cobra.Command, _ []string) error { ran = true; return nil }
	cmd.SetArgs([]string{"--skill-token=explicit-test-secret", "--callback-timeout=50s"})
	require.NoError(t, cmd.Execute())
	assert.True(t, ran)
}

func TestSkillServeRejectsUnboundedCallbackBudget(t *testing.T) {
	cmd := newSkillServeCommand(context.Background())
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--skill-token=test-secret", "--callback-timeout=2m"})
	require.ErrorContains(t, cmd.Execute(), "callback processing budget")
}
