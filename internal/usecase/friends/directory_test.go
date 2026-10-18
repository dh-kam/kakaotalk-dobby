package friends

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileFriendDirectory_SeedWhenNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	mapPath := filepath.Join(tmpDir, "friends_map.json")

	dir := NewFileFriendDirectory(mapPath)
	require.NotNil(t, dir)

	mappings := dir.All()
	assert.Len(t, mappings, 3)

	// Verify seed family accounts
	admin, ok := dir.FindByEmail("inzero@hanmail.net")
	require.True(t, ok)
	assert.True(t, admin.IsAdmin)
	assert.Equal(t, "MAQyBjICMQE5FSQUJB0sFCMWIQ08CToONwAzdA", admin.UUID)

	son, ok := dir.FindByEmail("kamtoyun@gmail.com")
	require.True(t, ok)
	assert.False(t, son.IsAdmin)

	mom, ok := dir.FindByEmail("sm81park@gmail.com")
	require.True(t, ok)
	assert.False(t, mom.IsAdmin)

	// Verify file was written to disk
	_, err := os.Stat(mapPath)
	assert.NoError(t, err)
}

func TestFileFriendDirectory_LookupAndGetAdmin(t *testing.T) {
	tmpDir := t.TempDir()
	mapPath := filepath.Join(tmpDir, "friends_map.json")

	dir := NewFileFriendDirectory(mapPath)

	// Case-insensitive email lookup
	mapping, ok := dir.FindByEmail("INZERO@HANMAIL.NET")
	assert.True(t, ok)
	assert.Equal(t, "아빠 (감동희)", mapping.Name)

	// Lookup by UUID
	mapping, ok = dir.FindByUUID("MAQyBjICMQE5FSQUJB0sFCMWIQ08CToONwAzdA")
	assert.True(t, ok)
	assert.Equal(t, "inzero@hanmail.net", mapping.Email)

	// Unknown UUID
	mapping, ok = dir.FindByUUID("UNKNOWN-UUID")
	assert.False(t, ok)
	assert.Nil(t, mapping)

	// GetAdmin returns inzero
	admin, ok := dir.GetAdmin()
	assert.True(t, ok)
	assert.Equal(t, "inzero@hanmail.net", admin.Email)
}

func TestFileFriendDirectory_AddOrUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	mapPath := filepath.Join(tmpDir, "friends_map.json")

	dir := NewFileFriendDirectory(mapPath)

	// Update existing mapping (son)
	err := dir.AddOrUpdate(FriendMapping{
		Name:    "아들 (감윤재 - 업데이트)",
		Email:   "kamtoyun@gmail.com",
		UUID:    "MAc2AzsLPQ82GigfKBEkHCsaIw8-CzgMNQIxZw",
		KakaoID: 5140656190,
		IsAdmin: true,
	})
	require.NoError(t, err)

	updatedSon, ok := dir.FindByEmail("kamtoyun@gmail.com")
	require.True(t, ok)
	assert.Equal(t, "아들 (감윤재 - 업데이트)", updatedSon.Name)
	assert.True(t, updatedSon.IsAdmin)

	// Add new mapping
	err = dir.AddOrUpdate(FriendMapping{
		Name:    "친구1",
		Email:   "friend1@example.com",
		UUID:    "NEW-UUID-9999",
		KakaoID: 9999999,
		IsAdmin: false,
	})
	require.NoError(t, err)

	assert.Len(t, dir.All(), 4)

	// Reload from disk to verify persistence
	reloaded := NewFileFriendDirectory(mapPath)
	assert.Len(t, reloaded.All(), 4)
	newFriend, ok := reloaded.FindByUUID("NEW-UUID-9999")
	require.True(t, ok)
	assert.Equal(t, "friend1@example.com", newFriend.Email)
}
