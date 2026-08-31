package scheduler

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduler_OnceAndCancel(t *testing.T) {
	var firedCount int32
	handler := func(ctx context.Context, job *Job) error {
		atomic.AddInt32(&firedCount, 1)
		return nil
	}

	store := NewMemoryStore()
	engine := NewEngine(store, handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, engine.Start(ctx))
	defer engine.Stop()

	// 1. Schedule a quick job (50ms)
	execTime := time.Now().Add(50 * time.Millisecond)
	job, err := engine.ScheduleOnce("user1", "테스트 알림", "내용", execTime, nil)
	require.NoError(t, err)
	assert.Equal(t, JobStatusActive, job.Status)

	time.Sleep(120 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&firedCount))

	updatedJob, err := engine.GetJob(job.ID)
	require.NoError(t, err)
	assert.Equal(t, JobStatusCompleted, updatedJob.Status)

	// 2. Schedule and cancel before firing
	execTime2 := time.Now().Add(200 * time.Millisecond)
	job2, err := engine.ScheduleOnce("user1", "취소될 알림", "내용", execTime2, nil)
	require.NoError(t, err)

	require.NoError(t, engine.CancelJob(job2.ID))
	time.Sleep(250 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&firedCount), "Cancelled job should not fire")
}

func TestScheduler_FileStorePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "jobs.json")

	store1, err := NewFileStore(filePath)
	require.NoError(t, err)

	var firedCount int32
	handler := func(ctx context.Context, job *Job) error {
		atomic.AddInt32(&firedCount, 1)
		return nil
	}

	engine1 := NewEngine(store1, handler)
	require.NoError(t, engine1.Start(context.Background()))

	// Schedule a job 100ms in the future
	execTime := time.Now().Add(100 * time.Millisecond)
	job, err := engine1.ScheduleOnce("user123", "지속성 테스트", "메시지", execTime, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, job.ID)

	// Simulate sudden restart before execution
	engine1.Stop()

	// Start a second engine with the same file
	store2, err := NewFileStore(filePath)
	require.NoError(t, err)

	engine2 := NewEngine(store2, handler)
	require.NoError(t, engine2.Start(context.Background()))
	defer engine2.Stop()

	// Wait for the restored timer to fire
	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&firedCount))

	savedJob, err := store2.Get(job.ID)
	require.NoError(t, err)
	assert.Equal(t, JobStatusCompleted, savedJob.Status)
}
