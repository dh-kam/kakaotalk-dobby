package agent

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// CompactorFunc compresses older conversation messages into a concise summary string.
type CompactorFunc func(messages []Message) string

// SessionStoreConfig defines parameters for conversation memory management.
type SessionStoreConfig struct {
	TTL         time.Duration
	MaxMessages int
	CompactTo   int
	Compactor   CompactorFunc
}

// SessionStore maintains multi-turn conversation histories per user session.
type SessionStore interface {
	GetHistory(sessionID string) []Message
	AppendTurn(sessionID string, userMsg string, assistantMsg string)
	Clear(sessionID string)
	SetCompactor(fn CompactorFunc)
}

type memorySessionStore struct {
	mu          sync.RWMutex
	ttl         time.Duration
	maxMessages int
	compactTo   int
	compactor   CompactorFunc
	sessions    map[string]*sessionData
}

type sessionData struct {
	lastActive time.Time
	messages   []Message
}

// NewMemorySessionStore creates an in-memory session manager with default TTL (15m), max 50 turns, and compact to 5 turns.
func NewMemorySessionStore(ttl time.Duration, maxMessages int) SessionStore {
	if maxMessages <= 0 {
		maxMessages = 50
	}
	return NewMemorySessionStoreWithConfig(SessionStoreConfig{
		TTL:         ttl,
		MaxMessages: maxMessages,
		CompactTo:   5,
	})
}

// NewMemorySessionStoreWithConfig creates a session store with full configuration.
func NewMemorySessionStoreWithConfig(cfg SessionStoreConfig) SessionStore {
	if cfg.TTL <= 0 {
		cfg.TTL = 15 * time.Minute
	}
	if cfg.MaxMessages <= 0 {
		cfg.MaxMessages = 50
	}
	if cfg.CompactTo <= 0 || cfg.CompactTo >= cfg.MaxMessages {
		cfg.CompactTo = 5
	}
	return &memorySessionStore{
		ttl:         cfg.TTL,
		maxMessages: cfg.MaxMessages,
		compactTo:   cfg.CompactTo,
		compactor:   cfg.Compactor,
		sessions:    make(map[string]*sessionData),
	}
}

func (s *memorySessionStore) SetCompactor(fn CompactorFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.compactor = fn
}

func (s *memorySessionStore) GetHistory(sessionID string) []Message {
	if sessionID == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked()

	sess, exists := s.sessions[sessionID]
	if !exists {
		return nil
	}

	if time.Since(sess.lastActive) > s.ttl {
		delete(s.sessions, sessionID)
		return nil
	}

	sess.lastActive = time.Now()

	out := make([]Message, len(sess.messages))
	copy(out, sess.messages)
	return out
}

func (s *memorySessionStore) AppendTurn(sessionID string, userMsg string, assistantMsg string) {
	if sessionID == "" || (userMsg == "" && assistantMsg == "") {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked()

	sess, exists := s.sessions[sessionID]
	if !exists {
		sess = &sessionData{
			lastActive: time.Now(),
			messages:   make([]Message, 0, s.maxMessages),
		}
		s.sessions[sessionID] = sess
	}

	sess.lastActive = time.Now()

	if userMsg != "" {
		sess.messages = append(sess.messages, Message{
			Role:    "user",
			Content: userMsg,
		})
	}

	if assistantMsg != "" {
		sess.messages = append(sess.messages, Message{
			Role:    "assistant",
			Content: assistantMsg,
		})
	}

	// Trigger compaction if messages exceed max threshold (50 turns -> 5 turns)
	if len(sess.messages) > s.maxMessages {
		s.compactSessionLocked(sess)
	}
}

func (s *memorySessionStore) compactSessionLocked(sess *sessionData) {
	if len(sess.messages) <= s.compactTo {
		return
	}

	// Reserve recent messages to keep intact (e.g. compactTo - 1 = 4 messages)
	keepRecent := s.compactTo - 1
	splitIdx := len(sess.messages) - keepRecent
	if splitIdx <= 0 {
		return
	}

	olderMsgs := sess.messages[:splitIdx]
	recentMsgs := sess.messages[splitIdx:]

	var summary string
	if s.compactor != nil {
		summary = s.compactor(olderMsgs)
	} else {
		summary = defaultCompactor(olderMsgs)
	}

	compactedHeader := Message{
		Role:    "assistant",
		Content: fmt.Sprintf("[이전 %d턴 대화 요약 맥락]\n%s", len(olderMsgs), summary),
	}

	newMessages := make([]Message, 0, s.compactTo)
	newMessages = append(newMessages, compactedHeader)
	newMessages = append(newMessages, recentMsgs...)

	sess.messages = newMessages
}

func defaultCompactor(messages []Message) string {
	var userTopics []string
	for _, m := range messages {
		if m.Role == "user" && m.Content != "" {
			trimmed := strings.TrimSpace(m.Content)
			if len(trimmed) > 40 {
				trimmed = trimmed[:40] + "..."
			}
			userTopics = append(userTopics, trimmed)
		}
	}

	if len(userTopics) > 8 {
		userTopics = userTopics[len(userTopics)-8:]
	}

	if len(userTopics) == 0 {
		return "이전 대화 맥락이 정리되었습니다."
	}

	return fmt.Sprintf("이전 주요 사용자 질의: %s", strings.Join(userTopics, " → "))
}

func (s *memorySessionStore) Clear(sessionID string) {
	if sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

func (s *memorySessionStore) cleanupExpiredLocked() {
	now := time.Now()
	for id, sess := range s.sessions {
		if now.Sub(sess.lastActive) > s.ttl {
			delete(s.sessions, id)
		}
	}
}
