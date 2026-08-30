package kakao

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileTokenStore_SaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	tokenPath := filepath.Join(tempDir, "tokens.json")

	store := NewFileTokenStore(tokenPath)

	_, err := store.Load(context.Background())
	assert.ErrorIs(t, err, ErrTokenNotFound)

	token := &TokenInfo{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "bearer",
		ExpiresIn:    3600,
		CreatedAt:    time.Now().Truncate(time.Second),
	}

	err = store.Save(context.Background(), token)
	require.NoError(t, err)

	loaded, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, token.AccessToken, loaded.AccessToken)
	assert.Equal(t, token.RefreshToken, loaded.RefreshToken)

	info, err := os.Stat(tokenPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}
