package kakao

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ErrTokenNotFound indicates that no stored token was found.
var ErrTokenNotFound = errors.New("kakao token not found")

// TokenStore defines persistence operations for OAuth tokens.
type TokenStore interface {
	Load(ctx context.Context) (*TokenInfo, error)
	Save(ctx context.Context, token *TokenInfo) error
}

// FileTokenStore stores tokens in a local JSON file.
type FileTokenStore struct {
	path string
	mu   sync.RWMutex
}

// DefaultTokenPath returns the default file path for token persistence (~/.config/kakao-bot/tokens.json).
func DefaultTokenPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "tokens.json"
	}
	return filepath.Join(home, ".config", "kakao-bot", "tokens.json")
}

// NewFileTokenStore creates a new FileTokenStore with the given file path.
func NewFileTokenStore(path string) *FileTokenStore {
	if path == "" {
		path = DefaultTokenPath()
	}
	return &FileTokenStore{
		path: path,
	}
}

// Load reads and parses TokenInfo from the token file.
func (s *FileTokenStore) Load(ctx context.Context) (*TokenInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("read token file %q: %w", s.path, err)
	}

	var token TokenInfo
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("unmarshal token: %w", err)
	}

	return &token, nil
}

// Save writes TokenInfo to the token file with secure permissions.
func (s *FileTokenStore) Save(ctx context.Context, token *TokenInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create token directory %q: %w", dir, err)
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("write token file %q: %w", s.path, err)
	}

	return nil
}

// MemoryTokenStore stores tokens in-memory (useful for tests).
type MemoryTokenStore struct {
	token *TokenInfo
	mu    sync.RWMutex
}

// NewMemoryTokenStore creates a memory-backed token store.
func NewMemoryTokenStore(initial *TokenInfo) *MemoryTokenStore {
	return &MemoryTokenStore{token: initial}
}

func (s *MemoryTokenStore) Load(ctx context.Context) (*TokenInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.token == nil {
		return nil, ErrTokenNotFound
	}
	cpy := *s.token
	return &cpy, nil
}

func (s *MemoryTokenStore) Save(ctx context.Context, token *TokenInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if token == nil {
		s.token = nil
		return nil
	}
	cpy := *token
	s.token = &cpy
	return nil
}
