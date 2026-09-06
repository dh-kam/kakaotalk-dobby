package kakao

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/samber/lo"
)

var (
	ErrBotNotFound = errors.New("bot not found")
	ErrNoBots      = errors.New("no bots registered")
)

// BotInfo provides read-only metadata about a registered bot.
type BotInfo struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
}

// ClientRegistry manages multiple Kakao client instances mapped by bot name.
type ClientRegistry interface {
	Register(name string, client Client, isDefault bool)
	Get(name string) (Client, error)
	GetDefault() (Client, error)
	SetDefault(name string) error
	List() []string
	Info() []BotInfo
	Count() int
}

type botRegistry struct {
	mu         sync.RWMutex
	clients    map[string]Client
	defaultBot string
}

// NewClientRegistry constructs a thread-safe bot registry.
func NewClientRegistry() ClientRegistry {
	return &botRegistry{
		clients: make(map[string]Client),
	}
}

func (r *botRegistry) Register(name string, client Client, isDefault bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		normalized = "default"
	}

	r.clients[normalized] = client
	if isDefault || len(r.clients) == 1 || r.defaultBot == "" {
		r.defaultBot = normalized
	}
}

func (r *botRegistry) Get(name string) (Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" || normalized == "default" {
		if r.defaultBot == "" {
			return nil, ErrNoBots
		}
		client, ok := r.clients[r.defaultBot]
		if !ok {
			return nil, fmt.Errorf("%w: default bot %q", ErrBotNotFound, r.defaultBot)
		}
		return client, nil
	}

	client, ok := r.clients[normalized]
	if !ok {
		available := lo.Keys(r.clients)
		sort.Strings(available)
		return nil, fmt.Errorf("%w: bot %q (available bots: %v)", ErrBotNotFound, normalized, available)
	}
	return client, nil
}

func (r *botRegistry) GetDefault() (Client, error) {
	return r.Get("")
}

func (r *botRegistry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	normalized := strings.ToLower(strings.TrimSpace(name))
	if _, ok := r.clients[normalized]; !ok {
		return fmt.Errorf("%w: cannot set %q as default", ErrBotNotFound, normalized)
	}
	r.defaultBot = normalized
	return nil
}

func (r *botRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := lo.Keys(r.clients)
	sort.Strings(names)
	return names
}

func (r *botRegistry) Info() []BotInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := lo.Keys(r.clients)
	sort.Strings(names)

	return lo.Map(names, func(name string, _ int) BotInfo {
		return BotInfo{
			Name:      name,
			IsDefault: name == r.defaultBot,
		}
	})
}

func (r *botRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

// BotTokenPath resolves the token file path for a specific bot.
func BotTokenPath(botName, customPath string) string {
	if customPath != "" {
		return customPath
	}
	normalized := strings.ToLower(strings.TrimSpace(botName))
	if normalized == "" || normalized == "default" {
		return DefaultTokenPath()
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Sprintf("tokens_%s.json", normalized)
	}
	return filepath.Join(home, ".config", "kakao-bot", fmt.Sprintf("tokens_%s.json", normalized))
}
