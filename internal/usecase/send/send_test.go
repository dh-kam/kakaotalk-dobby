package send

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestToken(t *testing.T) string {
	tempDir := t.TempDir()
	tokenPath := filepath.Join(tempDir, "tokens.json")
	store := kakao.NewFileTokenStore(tokenPath)
	err := store.Save(context.Background(), &kakao.TokenInfo{
		AccessToken: "mock-access-token",
		ExpiresIn:   3600,
		CreatedAt:   time.Now(),
	})
	require.NoError(t, err)
	return tokenPath
}

func TestSendMeUseCase_Validation(t *testing.T) {
	tokenPath := setupTestToken(t)
	uc := NewSendMeUseCase()

	// Empty text validation error
	err := uc.Execute(context.Background(), SendMeRequest{
		TokenPath: tokenPath,
		Text:      "",
		In:        strings.NewReader(""),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message text cannot be empty")
}
