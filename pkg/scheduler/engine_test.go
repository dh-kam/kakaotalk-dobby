package scheduler

import (
	"context"
	"path/filepath"
	"sync"
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

	require.NoError(t, engine.CancelJob(job2.ID, "user1"))
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

func TestScheduler_UpdateAndDelete(t *testing.T) {
	store := NewMemoryStore()
	engine := NewEngine(store, func(ctx context.Context, job *Job) error {
		return nil
	})
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop()

	// 1. Create
	execTime := time.Now().Add(1 * time.Hour)
	job, err := engine.ScheduleOnce("user1", "원본 제목", "원본 메시지", execTime, nil)
	require.NoError(t, err)

	// 2. Update Title & Message
	newTitle := "수정된 제목"
	newMsg := "수정된 메시지"
	updated, err := engine.UpdateJob(job.ID, JobUpdate{
		Title:   &newTitle,
		Message: &newMsg,
	}, "user1")
	require.NoError(t, err)
	assert.Equal(t, "수정된 제목", updated.Title)
	assert.Equal(t, "수정된 메시지", updated.Message)

	// Unauthorized update should fail
	_, err = engine.UpdateJob(job.ID, JobUpdate{Title: &newTitle}, "user_other")
	assert.Error(t, err)

	// Unauthorized delete should fail
	err = engine.DeleteJob(job.ID, "user_other")
	assert.Error(t, err)

	// 3. Authorized Delete
	err = engine.DeleteJob(job.ID, "user1")
	require.NoError(t, err)

	_, err = engine.GetJob(job.ID)
	assert.ErrorIs(t, err, ErrJobNotFound)
}

func TestScheduler_MultiTenantIsolation(t *testing.T) {
	store := NewMemoryStore()
	engine := NewEngine(store, nil)
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop()

	execTime := time.Now().Add(1 * time.Hour)
	_, err := engine.ScheduleOnce("user_alice", "Alice Job", "Msg", execTime, nil)
	require.NoError(t, err)

	_, err = engine.ScheduleOnce("user_bob", "Bob Job", "Msg", execTime, nil)
	require.NoError(t, err)

	aliceJobs := engine.ListJobs("user_alice")
	assert.Len(t, aliceJobs, 1)
	assert.Equal(t, "Alice Job", aliceJobs[0].Title)

	bobJobs := engine.ListJobs("user_bob")
	assert.Len(t, bobJobs, 1)
	assert.Equal(t, "Bob Job", bobJobs[0].Title)

	allJobs := engine.ListAllJobs()
	assert.Len(t, allJobs, 2)
}

func TestScheduler_ConcurrentScheduleAndCancel(t *testing.T) {
	store := NewMemoryStore()
	engine := NewEngine(store, func(ctx context.Context, job *Job) error {
		return nil
	})
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			execTime := time.Now().Add(50 * time.Millisecond)
			job, err := engine.ScheduleOnce("concurrent_user", "Title", "Msg", execTime, nil)
			if err == nil && idx%2 == 0 {
				_ = engine.CancelJob(job.ID, "concurrent_user")
			}
		}(i)
	}
	wg.Wait()
}
