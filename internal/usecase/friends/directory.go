package friends

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/samber/lo"
)

// FriendMapping associates a Kakao friend UUID and ID with a verified email and local profile name.
type FriendMapping struct {
	Name    string `json:"name"`               // Display name or role (e.g., "아빠 (감동희)")
	Email   string `json:"email"`              // Verified email address (e.g., "inzero@hanmail.net")
	UUID    string `json:"uuid"`               // KakaoTalk friend receiver UUID
	KakaoID int64  `json:"kakao_id,omitempty"` // Kakao User ID
	IsAdmin bool   `json:"is_admin,omitempty"` // True if this user should receive security & administrative alerts
}

// FriendDirectory manages friend identity mappings.
type FriendDirectory interface {
	All() []FriendMapping
	FindByUUID(uuid string) (*FriendMapping, bool)
	FindByEmail(email string) (*FriendMapping, bool)
	GetAdmin() (*FriendMapping, bool)
	AddOrUpdate(mapping FriendMapping) error
	Save() error
}

// DefaultFriendsMapPath returns ~/.config/kakao-bot/friends_map.json.
func DefaultFriendsMapPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "friends_map.json"
	}
	return filepath.Join(home, ".config", "kakao-bot", "friends_map.json")
}

// DefaultSeedMappings returns the initial baseline family accounts.
func DefaultSeedMappings() []FriendMapping {
	return []FriendMapping{
		{
			Name:    "아빠 (감동희)",
			Email:   "inzero@hanmail.net",
			UUID:    "MAQyBjICMQE5FSQUJB0sFCMWIQ08CToONwAzdA",
			KakaoID: 5064290474,
			IsAdmin: true,
		},
		{
			Name:    "아들 (감윤재)",
			Email:   "kamtoyun@gmail.com",
			UUID:    "MAc2AzsLPQ82GigfKBEkHCsaIw8-CzgMNQIxZw",
			KakaoID: 5140656190,
			IsAdmin: false,
		},
		{
			Name:    "엄마 (성미)",
			Email:   "sm81park@gmail.com",
			UUID:    "MAEzBzUMOA01GSgdKxkpHicUOAk8DzsCNQZW",
			KakaoID: 5140446071,
			IsAdmin: false,
		},
	}
}

// FileFriendDirectory implements FriendDirectory persisted to a local JSON file.
type FileFriendDirectory struct {
	path     string
	mu       sync.RWMutex
	mappings []FriendMapping
}

// NewFileFriendDirectory creates and initializes a FileFriendDirectory.
func NewFileFriendDirectory(path string) *FileFriendDirectory {
	if strings.TrimSpace(path) == "" {
		path = DefaultFriendsMapPath()
	}

	d := &FileFriendDirectory{
		path: path,
	}

	if err := d.load(); err != nil && os.IsNotExist(err) {
		// If the file does not exist, seed with default family mappings
		d.mappings = DefaultSeedMappings()
		_ = d.saveUnlocked()
	}

	return d
}

func (d *FileFriendDirectory) load() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	data, err := os.ReadFile(d.path)
	if err != nil {
		return err
	}

	var loaded []FriendMapping
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("unmarshal friends map %q: %w", d.path, err)
	}

	d.mappings = loaded
	return nil
}

// All returns a slice of all friend mappings.
func (d *FileFriendDirectory) All() []FriendMapping {
	d.mu.RLock()
	defer d.mu.RUnlock()

	copied := make([]FriendMapping, len(d.mappings))
	copy(copied, d.mappings)
	return copied
}

// FindByUUID looks up a mapping by Kakao receiver UUID.
func (d *FileFriendDirectory) FindByUUID(uuid string) (*FriendMapping, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	targetUUID := strings.TrimSpace(uuid)
	found, ok := lo.Find(d.mappings, func(m FriendMapping) bool {
		return strings.TrimSpace(m.UUID) == targetUUID
	})
	if !ok {
		return nil, false
	}
	res := found
	return &res, true
}

// FindByEmail looks up a mapping by email (case-insensitive).
func (d *FileFriendDirectory) FindByEmail(email string) (*FriendMapping, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	targetEmail := strings.ToLower(strings.TrimSpace(email))
	found, ok := lo.Find(d.mappings, func(m FriendMapping) bool {
		return strings.ToLower(strings.TrimSpace(m.Email)) == targetEmail
	})
	if !ok {
		return nil, false
	}
	res := found
	return &res, true
}

// GetAdmin returns the primary admin mapping, preferring IsAdmin==true or inzero@hanmail.net.
func (d *FileFriendDirectory) GetAdmin() (*FriendMapping, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// 1. Look for explicit IsAdmin flag
	if admin, ok := lo.Find(d.mappings, func(m FriendMapping) bool { return m.IsAdmin }); ok {
		res := admin
		return &res, true
	}

	// 2. Fallback to default inzero@hanmail.net
	if fallback, ok := lo.Find(d.mappings, func(m FriendMapping) bool {
		return strings.EqualFold(strings.TrimSpace(m.Email), "inzero@hanmail.net")
	}); ok {
		res := fallback
		return &res, true
	}

	return nil, false
}

// AddOrUpdate inserts or updates an existing mapping by UUID or Email.
func (d *FileFriendDirectory) AddOrUpdate(mapping FriendMapping) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	cleanUUID := strings.TrimSpace(mapping.UUID)
	cleanEmail := strings.ToLower(strings.TrimSpace(mapping.Email))

	updated := false
	for i, existing := range d.mappings {
		if (cleanUUID != "" && strings.TrimSpace(existing.UUID) == cleanUUID) ||
			(cleanEmail != "" && strings.ToLower(strings.TrimSpace(existing.Email)) == cleanEmail) {
			d.mappings[i] = mapping
			updated = true
			break
		}
	}

	if !updated {
		d.mappings = append(d.mappings, mapping)
	}

	return d.saveUnlocked()
}

// Save writes current mappings to disk.
func (d *FileFriendDirectory) Save() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.saveUnlocked()
}

func (d *FileFriendDirectory) saveUnlocked() error {
	if d.path == "" {
		return fmt.Errorf("friends map path is empty")
	}

	dir := filepath.Dir(d.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config directory %q: %w", dir, err)
	}

	bytes, err := json.MarshalIndent(d.mappings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal friends map: %w", err)
	}

	tmpFile := d.path + ".tmp"
	if err := os.WriteFile(tmpFile, bytes, 0600); err != nil {
		return fmt.Errorf("write temp file %q: %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, d.path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("atomic rename friends map: %w", err)
	}

	return nil
}
