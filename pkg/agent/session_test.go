package agent

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSessionStore_Compaction(t *testing.T) {
	// Configure store with maxMessages = 10, compactTo = 6 (2 summary + 4 recent)
	store := NewMemorySessionStoreWithConfig(SessionStoreConfig{
		TTL:         0,
		MaxMessages: 10,
		CompactTo:   6,
	})

	// Add 5 turns (10 messages: 5 user + 5 assistant)
	for i := 1; i <= 5; i++ {
		store.AppendTurn("user1", fmt.Sprintf("질문 %d", i), fmt.Sprintf("답변 %d", i))
	}

	history := store.GetHistory("user1")
	assert.Len(t, history, 10)
	assert.Equal(t, "질문 1", history[0].Content)
	assert.Equal(t, "답변 5", history[9].Content)

	// Add 6th turn (total 12 messages -> triggers compaction to 6)
	store.AppendTurn("user1", "질문 6", "답변 6")

	compactedHistory := store.GetHistory("user1")
	assert.Len(t, compactedHistory, 6)

	// Message 0 and 1 should alternate user and assistant for LLM protocol compliance
	assert.Equal(t, "user", compactedHistory[0].Role)
	assert.Contains(t, compactedHistory[0].Content, "대화 요약 맥락")
	assert.Contains(t, compactedHistory[0].Content, "질문 1")
	assert.Equal(t, "assistant", compactedHistory[1].Role)

	// Messages 2..5 should be the recent turns (질문 5, 답변 5, 질문 6, 답변 6)
	assert.Equal(t, "질문 5", compactedHistory[2].Content)
	assert.Equal(t, "답변 5", compactedHistory[3].Content)
	assert.Equal(t, "질문 6", compactedHistory[4].Content)
	assert.Equal(t, "답변 6", compactedHistory[5].Content)
}

func TestSessionStore_Default50TurnsCompaction(t *testing.T) {
	// Default store: maxMessages = 100 (50 turns), compactTo = 12 (5 turns + summary pair), no TTL
	store := NewMemorySessionStore(100)

	// Append 50 turns (100 messages)
	for i := 1; i <= 50; i++ {
		store.AppendTurn("user_50", fmt.Sprintf("대화 %d", i), fmt.Sprintf("응답 %d", i))
	}

	h100 := store.GetHistory("user_50")
	assert.Len(t, h100, 100)

	// Append 51st turn (102 messages -> triggers compaction to 12)
	store.AppendTurn("user_50", "대화 51", "응답 51")

	hCompacted := store.GetHistory("user_50")
	assert.Len(t, hCompacted, 12)
	assert.Equal(t, "user", hCompacted[0].Role)
	assert.Contains(t, hCompacted[0].Content, "대화 요약 맥락")
	assert.Equal(t, "assistant", hCompacted[1].Role)

	// Last turns should be preserved
	assert.Equal(t, "대화 51", hCompacted[10].Content)
	assert.Equal(t, "응답 51", hCompacted[11].Content)
}

func TestSessionStore_NoInactivityExpiration(t *testing.T) {
	// Default persistent store
	store := NewMemorySessionStore(50)
	store.AppendTurn("persistent_user", "안녕", "반가워요")

	// Even if time passes, it should not be cleared
	h := store.GetHistory("persistent_user")
	assert.Len(t, h, 2)
	assert.Equal(t, "안녕", h[0].Content)
}

func TestSessionStore_CustomCompactor(t *testing.T) {
	store := NewMemorySessionStoreWithConfig(SessionStoreConfig{
		TTL:         0,
		MaxMessages: 6,
		CompactTo:   4,
		Compactor: func(msgs []Message) string {
			return "🎯 사용자 맞춤형 컴팩션 완료"
		},
	})

	store.AppendTurn("user_custom", "질문 1", "답변 1")
	store.AppendTurn("user_custom", "질문 2", "답변 2")
	store.AppendTurn("user_custom", "질문 3", "답변 3")
	store.AppendTurn("user_custom", "질문 4", "답변 4") // triggers compaction (8 > 6 -> 4)

	h := store.GetHistory("user_custom")
	assert.Len(t, h, 4)
	assert.Contains(t, h[0].Content, "🎯 사용자 맞춤형 컴팩션 완료")
}

