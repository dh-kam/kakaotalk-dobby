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
	TTL         time.Duration // If <= 0, sessions never expire based on time (persistent in-memory)
	MaxSessions int           // Maximum concurrent sessions before LRU eviction (default 1000)
	MaxMessages int           // Threshold messages before compaction (default 50)
	CompactTo   int           // Number of messages after compaction (default 5)
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
	maxSessions int
	maxMessages int
	compactTo   int
	compactor   CompactorFunc
	sessions    map[string]*sessionData
}

type sessionData struct {
	lastActive time.Time
	messages   []Message
}

const (
	defaultMaxSessions   = 1000
	defaultMaxMessages   = 50
	defaultCompactTo     = 5
	maxMessageRuneLength = 8000
)

// NewMemorySessionStore creates an in-memory session manager with persistent memory (no TTL expiration), max 50 messages, and compact to 5 messages.
func NewMemorySessionStore(maxMessages int) SessionStore {
	if maxMessages <= 0 {
		maxMessages = defaultMaxMessages
	}
	return NewMemorySessionStoreWithConfig(SessionStoreConfig{
		TTL:         0, // No time-based expiration
		MaxSessions: defaultMaxSessions,
		MaxMessages: maxMessages,
		CompactTo:   defaultCompactTo,
	})
}

// NewMemorySessionStoreWithConfig creates a session store with full configuration.
func NewMemorySessionStoreWithConfig(cfg SessionStoreConfig) SessionStore {
	if cfg.MaxSessions <= 0 {
		cfg.MaxSessions = defaultMaxSessions
	}
	if cfg.MaxMessages <= 0 {
		cfg.MaxMessages = defaultMaxMessages
	}
	if cfg.CompactTo <= 0 || cfg.CompactTo >= cfg.MaxMessages {
		cfg.CompactTo = defaultCompactTo
	}
	return &memorySessionStore{
		ttl:         cfg.TTL,
		maxSessions: cfg.MaxSessions,
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

	if s.ttl > 0 && time.Since(sess.lastActive) > s.ttl {
		delete(s.sessions, sessionID)
		return nil
	}

	sess.lastActive = time.Now()

	out := make([]Message, len(sess.messages))
	copy(out, sess.messages)
	return out
}

func sanitizeMessageContent(content string) string {
	runes := []rune(content)
	if len(runes) > maxMessageRuneLength {
		return string(runes[:maxMessageRuneLength]) + " (truncated)"
	}
	return content
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
		// Enforce LRU eviction if maximum sessions reached
		if len(s.sessions) >= s.maxSessions {
			s.evictLRULocked()
		}

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
			Content: sanitizeMessageContent(userMsg),
		})
	}

	if assistantMsg != "" {
		sess.messages = append(sess.messages, Message{
			Role:    "assistant",
			Content: sanitizeMessageContent(assistantMsg),
		})
	}

	// Trigger compaction if messages exceed max threshold (50 -> 5)
	if len(sess.messages) > s.maxMessages {
		s.compactSessionLocked(sess)
	}
}

func (s *memorySessionStore) evictLRULocked() {
	var oldestID string
	var oldestTime time.Time

	first := true
	for id, sess := range s.sessions {
		if first || sess.lastActive.Before(oldestTime) {
			oldestTime = sess.lastActive
			oldestID = id
			first = false
		}
	}

	if oldestID != "" {
		delete(s.sessions, oldestID)
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
			runes := []rune(trimmed)
			if len(runes) > 40 {
				trimmed = string(runes[:40]) + "..."
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
	if s.ttl <= 0 {
		return
	}
	now := time.Now()
	for id, sess := range s.sessions {
		if now.Sub(sess.lastActive) > s.ttl {
			delete(s.sessions, id)
		}
	}
}
