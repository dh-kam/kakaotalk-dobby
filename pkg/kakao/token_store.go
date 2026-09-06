package kakao

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrTokenNotFound = errors.New("kakao token not found")

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

// NewFileTokenStore creates a new FileTokenStore.
func NewFileTokenStore(path string) *FileTokenStore {
	if path == "" {
		path = DefaultTokenPath()
	}
	return &FileTokenStore{path: path}
}

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

	// Atomic write using temp file and rename to prevent corrupted tokens on crash
	tmpFile := fmt.Sprintf("%s.tmp.%d", s.path, time.Now().UnixNano())
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create temp token file: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("write temp token file: %w", err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("sync temp token file: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("close temp token file: %w", err)
	}

	if err := os.Rename(tmpFile, s.path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("rename token file %q: %w", s.path, err)
	}

	return nil
}

func (s *FileTokenStore) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove token file %q: %w", s.path, err)
	}
	return nil
}

// MemoryTokenStore stores tokens in-memory.
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

func (s *MemoryTokenStore) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = nil
	return nil
}
