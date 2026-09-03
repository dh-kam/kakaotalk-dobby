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
	defaultMaxMessages   = 100
	defaultCompactTo     = 12
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
		if cfg.MaxMessages > defaultCompactTo {
			cfg.CompactTo = defaultCompactTo
		} else {
			cfg.CompactTo = cfg.MaxMessages / 2
			if cfg.CompactTo < 2 {
				cfg.CompactTo = 2
			}
		}
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

	// Reserve recent complete turns to keep intact (compactTo - 2 reserved for summary pair)
	keepRecent := s.compactTo - 2
	if keepRecent < 2 {
		keepRecent = 2
	}
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

	// Strictly alternate user and assistant roles for summary context
	// This satisfies strict role requirements in Claude/Anthropic, Bedrock, and Gemini
	compactedUser := Message{
		Role:    "user",
		Content: fmt.Sprintf("[이전 대화 요약 맥락 (%d개 발화 요약)]\n%s", len(olderMsgs), summary),
	}
	compactedAssistant := Message{
		Role:    "assistant",
		Content: "네, 이전 대화에서 확인된 핵심 정보(정류장, 시간표, 일정 등)를 모두 기억하고 있습니다. 계속해서 말씀해 주세요!",
	}

	newMessages := make([]Message, 0, len(recentMsgs)+2)
	newMessages = append(newMessages, compactedUser, compactedAssistant)
	newMessages = append(newMessages, recentMsgs...)

	sess.messages = newMessages
}

func defaultCompactor(messages []Message) string {
	var userQueries []string
	var entities []string
	entitySet := make(map[string]struct{})

	addEntity := func(e string) {
		if _, exists := entitySet[e]; !exists {
			entitySet[e] = struct{}{}
			entities = append(entities, e)
		}
	}

	// Keyword extractors for known important domain entities
	knownLocations := []string{"우미린 2차", "우미린2차", "양포도서관", "구미확장우미린", "산동", "옥계", "해마루", "현진", "이편한", "중흥", "호반", "원당"}
	knownTimes := []string{"3시 40분", "5시 20분", "3:40", "5:20", "15:40", "17:20", "15:15", "15:20", "15:25", "15:30"}

	for _, m := range messages {
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}

		// Detect and preserve known entities across both user and assistant
		for _, loc := range knownLocations {
			if strings.Contains(content, loc) {
				addEntity("위치/정류장: " + loc)
			}
		}
		for _, tm := range knownTimes {
			if strings.Contains(content, tm) {
				addEntity("수업/탑승시각: " + tm)
			}
		}
		if strings.Contains(content, "정상어학원") || strings.Contains(content, "2호차") {
			addEntity("학원/노선: 정상어학원 2호차")
		}
		if strings.Contains(content, "6학년") || strings.Contains(content, "9반") {
			addEntity("학교/학급: 6학년 9반")
		}

		// Collect user questions
		if m.Role == "user" {
			// If message already contains previous summary header, skip re-adding as raw user query
			if strings.HasPrefix(content, "[이전 대화 요약 맥락") {
				continue
			}
			runes := []rune(content)
			if len(runes) > 50 {
				content = string(runes[:50]) + "..."
			}
			userQueries = append(userQueries, content)
		}
	}

	var sb strings.Builder
	if len(entities) > 0 {
		sb.WriteString("📌 확인된 주요 엔티티 및 사실:\n")
		for _, e := range entities {
			sb.WriteString(fmt.Sprintf("• %s\n", e))
		}
		sb.WriteString("\n")
	}

	if len(userQueries) > 0 {
		if len(userQueries) > 6 {
			userQueries = userQueries[len(userQueries)-6:]
		}
		sb.WriteString(fmt.Sprintf("💬 이전 주요 질문 흐름: %s", strings.Join(userQueries, " ➔ ")))
	} else {
		sb.WriteString("이전 대화 맥락이 성공적으로 요약 정리되었습니다.")
	}

	return strings.TrimSpace(sb.String())
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
