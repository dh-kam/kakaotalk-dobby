package agent

import (
	"strings"
	"sync"
)

// ModelInfo describes an available AI model for user selection.
type ModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Provider    string `json:"provider"`
}

// AvailableModels lists all supported models that users can explicitly select.
var AvailableModels = []ModelInfo{
	{
		ID:          "gemini-3.8-flash",
		Name:        "Gemini 3.8 Flash",
		Description: "Google 최신 플래시 모델 (초저지연, ~2026.12.31 가성비 1위)",
		Provider:    "Google Vertex AI",
	},
	{
		ID:          "grok-4.20-non-reasoning",
		Name:        "Grok 4.20 Non-Reasoning",
		Description: "xAI 고속 직답형 모델 (저렴한 출력 단가, 2027.01.01~ 기본 모델)",
		Provider:    "xAI / Vertex AI",
	},
	{
		ID:          "grok-4.3",
		Name:        "Grok 4.3",
		Description: "xAI 최신 플래그십 모델 (복잡한 코딩, 수식, 심층 추론)",
		Provider:    "xAI / Vertex AI",
	},
}

// ModelPreferenceStore manages user-selected AI model preferences.
type ModelPreferenceStore interface {
	Get(userID string) string
	Set(userID string, model string)
	Reset(userID string)
	ListAll() map[string]string
}

type inMemoryModelPreferenceStore struct {
	mu          sync.RWMutex
	preferences map[string]string
}

var (
	defaultPrefStoreOnce sync.Once
	defaultPrefStore     ModelPreferenceStore
)

// GetDefaultModelPreferenceStore returns the singleton model preference store.
func GetDefaultModelPreferenceStore() ModelPreferenceStore {
	defaultPrefStoreOnce.Do(func() {
		defaultPrefStore = &inMemoryModelPreferenceStore{
			preferences: make(map[string]string),
		}
	})
	return defaultPrefStore
}

func (s *inMemoryModelPreferenceStore) Get(userID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.preferences[strings.TrimSpace(userID)]
}

func (s *inMemoryModelPreferenceStore) Set(userID string, model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cleanUser := strings.TrimSpace(userID)
	cleanModel := strings.TrimSpace(model)
	if cleanModel == "" || cleanModel == "auto" || cleanModel == "reset" {
		delete(s.preferences, cleanUser)
		return
	}
	s.preferences[cleanUser] = cleanModel
}

func (s *inMemoryModelPreferenceStore) Reset(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.preferences, strings.TrimSpace(userID))
}

func (s *inMemoryModelPreferenceStore) ListAll() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copied := make(map[string]string, len(s.preferences))
	for k, v := range s.preferences {
		copied[k] = v
	}
	return copied
}

// NormalizeModelAlias normalizes user input aliases to canonical model IDs.
func NormalizeModelAlias(input string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(input))
	switch lower {
	case "gemini", "flash", "3.8", "gemini-flash", "gemini-3.8-flash", "1":
		return "gemini-3.8-flash", true
	case "grok", "grok4", "grok4.2", "4.2", "4.20", "grok-4.20", "grok-4.20-non-reasoning", "2":
		return "grok-4.20-non-reasoning", true
	case "grok4.3", "4.3", "grok-4.3", "3":
		return "grok-4.3", true
	case "auto", "reset", "기본", "초기화", "자동", "0":
		return "auto", true
	default:
		// Check exact match in AvailableModels
		for _, m := range AvailableModels {
			if strings.EqualFold(m.ID, lower) {
				return m.ID, true
			}
		}
		return "", false
	}
}
