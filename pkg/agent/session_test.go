package agent

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSessionStore_Compaction(t *testing.T) {
	// Configure store with maxMessages = 10, compactTo = 5
	store := NewMemorySessionStoreWithConfig(SessionStoreConfig{
		TTL:         10 * time.Minute,
		MaxMessages: 10,
		CompactTo:   5,
	})

	// Add 5 turns (10 messages: 5 user + 5 assistant)
	for i := 1; i <= 5; i++ {
		store.AppendTurn("user1", fmt.Sprintf("질문 %d", i), fmt.Sprintf("답변 %d", i))
	}

	history := store.GetHistory("user1")
	assert.Len(t, history, 10)
	assert.Equal(t, "질문 1", history[0].Content)
	assert.Equal(t, "답변 5", history[9].Content)

	// Add 6th turn (total 12 messages -> triggers compaction to 5)
	store.AppendTurn("user1", "질문 6", "답변 6")

	compactedHistory := store.GetHistory("user1")
	assert.Len(t, compactedHistory, 5)

	// Message 0 should be the compaction summary header
	assert.Equal(t, "assistant", compactedHistory[0].Role)
	assert.Contains(t, compactedHistory[0].Content, "[이전 8턴 대화 요약 맥락]")
	assert.Contains(t, compactedHistory[0].Content, "질문 1")

	// Messages 1..4 should be the recent turns (질문 5, 답변 5, 질문 6, 답변 6)
	assert.Equal(t, "질문 5", compactedHistory[1].Content)
	assert.Equal(t, "답변 5", compactedHistory[2].Content)
	assert.Equal(t, "질문 6", compactedHistory[3].Content)
	assert.Equal(t, "답변 6", compactedHistory[4].Content)
}

func TestSessionStore_Default50TurnsCompaction(t *testing.T) {
	// Default store: maxMessages = 50, compactTo = 5
	store := NewMemorySessionStore(10*time.Minute, 50)

	// Append 25 turns (50 messages)
	for i := 1; i <= 25; i++ {
		store.AppendTurn("user_50", fmt.Sprintf("대화 %d", i), fmt.Sprintf("응답 %d", i))
	}

	h50 := store.GetHistory("user_50")
	assert.Len(t, h50, 50)

	// Append 26th turn (52 messages -> triggers compaction to 5)
	store.AppendTurn("user_50", "대화 26", "응답 26")

	hCompacted := store.GetHistory("user_50")
	assert.Len(t, hCompacted, 5)
	assert.Contains(t, hCompacted[0].Content, "대화 요약 맥락")
	assert.Equal(t, "대화 25", hCompacted[1].Content)
	assert.Equal(t, "응답 25", hCompacted[2].Content)
	assert.Equal(t, "대화 26", hCompacted[3].Content)
	assert.Equal(t, "응답 26", hCompacted[4].Content)
}

func TestSessionStore_CustomCompactor(t *testing.T) {
	store := NewMemorySessionStoreWithConfig(SessionStoreConfig{
		TTL:         10 * time.Minute,
		MaxMessages: 4,
		CompactTo:   3,
		Compactor: func(msgs []Message) string {
			return "🎯 사용자 맞춤형 컴팩션 완료"
		},
	})

	store.AppendTurn("user_custom", "질문 1", "답변 1")
	store.AppendTurn("user_custom", "질문 2", "답변 2")
	store.AppendTurn("user_custom", "질문 3", "답변 3") // triggers compaction

	h := store.GetHistory("user_custom")
	assert.Len(t, h, 3)
	assert.Contains(t, h[0].Content, "🎯 사용자 맞춤형 컴팩션 완료")
}
