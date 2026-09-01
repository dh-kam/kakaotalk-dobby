package agent

import (
	"sync"
	"time"
)

// SessionStore maintains multi-turn conversation histories per user session.
type SessionStore interface {
	GetHistory(sessionID string) []Message
	AppendTurn(sessionID string, userMsg string, assistantMsg string)
	Clear(sessionID string)
}

type memorySessionStore struct {
	mu          sync.RWMutex
	ttl         time.Duration
	maxMessages int
	sessions    map[string]*sessionData
}

type sessionData struct {
	lastActive time.Time
	messages   []Message
}

// NewMemorySessionStore creates an in-memory session manager with TTL and max message retention.
func NewMemorySessionStore(ttl time.Duration, maxMessages int) SessionStore {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	if maxMessages <= 0 {
		maxMessages = 10
	}
	return &memorySessionStore{
		ttl:         ttl,
		maxMessages: maxMessages,
		sessions:    make(map[string]*sessionData),
	}
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

	// Return a copy of messages
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

	// Maintain sliding window of messages
	if len(sess.messages) > s.maxMessages {
		sess.messages = sess.messages[len(sess.messages)-s.maxMessages:]
	}
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
