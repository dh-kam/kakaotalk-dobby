package agent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSessionStore_SlidingWindowAndTTL(t *testing.T) {
	store := NewMemorySessionStore(100*time.Millisecond, 4)

	// Append turns
	store.AppendTurn("user1", "안녕", "안녕하세요!")
	store.AppendTurn("user1", "정상어학원 버스", "몇 시 수업이신가요?")

	history := store.GetHistory("user1")
	assert.Len(t, history, 4)
	assert.Equal(t, "user", history[0].Role)
	assert.Equal(t, "안녕", history[0].Content)
	assert.Equal(t, "assistant", history[3].Role)
	assert.Equal(t, "몇 시 수업이신가요?", history[3].Content)

	// Append another turn -> sliding window should drop oldest 2
	store.AppendTurn("user1", "3시 40분", "3시 40분 버스 시간표입니다.")
	history2 := store.GetHistory("user1")
	assert.Len(t, history2, 4)
	assert.Equal(t, "정상어학원 버스", history2[0].Content)
	assert.Equal(t, "3시 40분 버스 시간표입니다.", history2[3].Content)

	// Test Clear
	store.Clear("user1")
	assert.Nil(t, store.GetHistory("user1"))

	// Test TTL Expiration
	store.AppendTurn("user2", "테스트", "응답")
	assert.Len(t, store.GetHistory("user2"), 2)
	time.Sleep(120 * time.Millisecond)
	assert.Nil(t, store.GetHistory("user2"))
}
