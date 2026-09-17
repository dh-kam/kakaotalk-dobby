package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInFlightExecutionPreservesCommittedMutations(t *testing.T) {
	for _, fileBacked := range []bool{false, true} {
		for _, kind := range []ScheduleType{ScheduleTypeOnce, ScheduleTypeRecurring} {
			for _, action := range []string{"delete", "cancel", "metadata", "reschedule"} {
				name := string(kind) + "/" + action
				if fileBacked {
					name = "file/" + name
				} else {
					name = "memory/" + name
				}
				t.Run(name, func(t *testing.T) {
					path := filepath.Join(t.TempDir(), "jobs.json")
					var store Store = NewMemoryStore()
					if fileBacked {
						fileStore, err := NewFileStore(path)
						require.NoError(t, err)
						store = fileStore
					}
					started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
					unblock := sync.OnceFunc(func() { close(release) })
					defer unblock()
					engine := NewEngine(store, func(context.Context, *Job) error {
						close(started)
						<-release
						return nil
					})
					require.NoError(t, engine.Start(t.Context()))
					t.Cleanup(engine.Stop)
					var job *Job
					var err error
					if kind == ScheduleTypeOnce {
						job, err = engine.ScheduleOnce("alice", "original", "message", time.Now().Add(time.Hour), nil)
					} else {
						job, err = engine.ScheduleRecurring("alice", "original", "message", "@every 1h", nil)
					}
					require.NoError(t, err)
					go func() { engine.executeJob(job.ID, job.Revision, engine.ctx); close(done) }()
					select {
					case <-started:
					case <-time.After(time.Second):
						t.Fatal("job never entered handler")
					}
					future := time.Now().Add(2 * time.Hour)
					switch action {
					case "delete":
						require.NoError(t, engine.DeleteJob(job.ID, "alice"))
					case "cancel":
						require.NoError(t, engine.CancelJob(job.ID, "alice"))
					case "metadata":
						title := "updated"
						_, err = engine.UpdateJob(job.ID, JobUpdate{Title: &title}, "alice")
						require.NoError(t, err)
					case "reschedule":
						_, err = engine.UpdateJob(job.ID, JobUpdate{ExecuteAt: &future}, "alice")
						require.NoError(t, err)
					}
					unblock()
					select {
					case <-done:
					case <-time.After(time.Second):
						t.Fatal("job did not finish")
					}
					// Check the durable state as well as memory, so a process restart
					// cannot resurrect a cancelled/deleted recurring notification.
					if fileBacked {
						store, err = NewFileStore(path)
						require.NoError(t, err)
					}
					after, err := store.Get(job.ID)
					if action == "delete" {
						require.ErrorIs(t, err, ErrJobNotFound)
						return
					}
					require.NoError(t, err)
					switch action {
					case "cancel":
						assert.Equal(t, JobStatusCancelled, after.Status)
					case "metadata":
						assert.Equal(t, "updated", after.Title)
						assert.NotNil(t, after.LastFiredAt)
						if kind == ScheduleTypeOnce {
							assert.Equal(t, JobStatusCompleted, after.Status)
						}
					case "reschedule":
						assert.Equal(t, JobStatusActive, after.Status)
						assert.True(t, future.Equal(after.ExecuteAt))
						assert.Nil(t, after.LastFiredAt)
					}
				})
			}
		}
	}
}

type failingMutationStore struct {
	*MemoryStore
	failSave, failDelete bool
}

func (s *failingMutationStore) Save(job *Job) error {
	if s.failSave {
		return errors.New("simulated disk full")
	}
	return s.MemoryStore.Save(job)
}
func (s *failingMutationStore) Delete(id string) error {
	if s.failDelete {
		return errors.New("simulated read-only disk")
	}
	return s.MemoryStore.Delete(id)
}

func TestFailedMutationPreservesStoredJobAndTrigger(t *testing.T) {
	for _, kind := range []ScheduleType{ScheduleTypeOnce, ScheduleTypeRecurring} {
		for _, action := range []string{"cancel", "update", "delete"} {
			t.Run(string(kind)+"/"+action, func(t *testing.T) {
				store := &failingMutationStore{MemoryStore: NewMemoryStore()}
				engine := NewEngine(store, nil)
				require.NoError(t, engine.Start(t.Context()))
				defer engine.Stop()
				var job *Job
				var err error
				if kind == ScheduleTypeOnce {
					job, err = engine.ScheduleOnce("alice", "title", "message", time.Now().Add(time.Hour), nil)
				} else {
					job, err = engine.ScheduleRecurring("alice", "title", "message", "@every 1h", nil)
				}
				require.NoError(t, err)
				timer, entry := engine.timerMap[job.ID], engine.entryMap[job.ID]
				store.failSave, store.failDelete = true, true
				switch action {
				case "cancel":
					err = engine.CancelJob(job.ID, "alice")
				case "delete":
					err = engine.DeleteJob(job.ID, "alice")
				case "update":
					future := time.Now().Add(2 * time.Hour)
					_, err = engine.UpdateJob(job.ID, JobUpdate{ExecuteAt: &future}, "alice")
				}
				require.Error(t, err)
				after, err := engine.GetJob(job.ID)
				require.NoError(t, err)
				assert.Equal(t, job, after)
				if kind == ScheduleTypeOnce {
					assert.Same(t, timer, engine.timerMap[job.ID])
				} else {
					assert.Equal(t, entry, engine.entryMap[job.ID])
				}
			})
		}
	}
}

func TestInvalidCronLeavesExistingScheduleIntact(t *testing.T) {
	engine := NewEngine(NewMemoryStore(), nil)
	require.NoError(t, engine.Start(t.Context()))
	defer engine.Stop()
	job, err := engine.ScheduleRecurring("alice", "original", "message", "@every 1h", nil)
	require.NoError(t, err)
	entry := engine.entryMap[job.ID]
	invalid := "not a cron expression"
	_, err = engine.UpdateJob(job.ID, JobUpdate{CronExpr: &invalid}, "alice")
	require.Error(t, err)
	after, err := engine.GetJob(job.ID)
	require.NoError(t, err)
	assert.Equal(t, job, after)
	assert.Equal(t, entry, engine.entryMap[job.ID])
	assert.NotZero(t, engine.cronRunner.Entry(entry).ID)

	stopped := NewEngine(NewMemoryStore(), nil)
	_, err = stopped.ScheduleRecurring("alice", "invalid", "message", invalid, nil)
	require.Error(t, err)
	assert.Empty(t, stopped.ListAllJobs())
}

func TestStaleTriggerCannotExecuteNewSchedule(t *testing.T) {
	var calls atomic.Int32
	engine := NewEngine(NewMemoryStore(), func(context.Context, *Job) error { calls.Add(1); return nil })
	require.NoError(t, engine.Start(t.Context()))
	defer engine.Stop()
	job, err := engine.ScheduleOnce("alice", "title", "message", time.Now().Add(time.Hour), nil)
	require.NoError(t, err)
	future := time.Now().Add(2 * time.Hour)
	updated, err := engine.UpdateJob(job.ID, JobUpdate{ExecuteAt: &future}, "alice")
	require.NoError(t, err)
	engine.executeJob(job.ID, job.Revision, engine.ctx)
	assert.Zero(t, calls.Load())
	assert.Contains(t, engine.timerMap, job.ID)
	engine.executeJob(updated.ID, updated.Revision, engine.ctx)
	assert.Equal(t, int32(1), calls.Load())
}

func TestJobIdentityIsExactAndHasNoWildcard(t *testing.T) {
	engine := NewEngine(NewMemoryStore(), nil)
	job, err := engine.ScheduleOnce("alice", "private", "message", time.Now().Add(time.Hour), nil)
	require.NoError(t, err)
	for _, user := range []string{"", "*", " ", " alice ", "Alice", "bob"} {
		assert.Empty(t, engine.ListJobs(user), "user %q", user)
		assert.Error(t, engine.CancelJob(job.ID, user), "user %q", user)
		assert.Error(t, engine.DeleteJob(job.ID, user), "user %q", user)
		title := "tampered"
		_, err = engine.UpdateJob(job.ID, JobUpdate{Title: &title}, user)
		assert.Error(t, err, "user %q", user)
	}
	assert.Len(t, engine.ListJobs("alice"), 1)
	assert.Len(t, engine.ListAllJobs(), 1)
}

func TestLegacyJobWithoutRevisionCanBeRestoredAndCancelled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	data := fmt.Sprintf(`[{"id":"legacy-job","user_id":"alice","title":"legacy","type":"recurring","cron_expr":"@every 1h","status":"active","created_at":%q}]`, time.Now().Format(time.RFC3339))
	require.NoError(t, os.WriteFile(path, []byte(data), 0600))
	store, err := NewFileStore(path)
	require.NoError(t, err)
	engine := NewEngine(store, nil)
	require.NoError(t, engine.Start(t.Context()))
	t.Cleanup(engine.Stop)
	job, err := engine.GetJob("legacy-job")
	require.NoError(t, err)
	assert.Zero(t, job.Revision)
	assert.Contains(t, engine.entryMap, job.ID)
	require.NoError(t, engine.CancelJob(job.ID, "alice"))
	engine.Stop()

	restored, err := NewFileStore(path)
	require.NoError(t, err)
	restarted := NewEngine(restored, nil)
	require.NoError(t, restarted.Start(t.Context()))
	defer restarted.Stop()
	job, err = restarted.GetJob("legacy-job")
	require.NoError(t, err)
	assert.Equal(t, JobStatusCancelled, job.Status)
	assert.Empty(t, restarted.entryMap)
}

func TestRestartRemovesOldCronEntriesAndIgnoresOldTriggers(t *testing.T) {
	var calls atomic.Int32
	engine := NewEngine(NewMemoryStore(), func(context.Context, *Job) error { calls.Add(1); return nil })
	require.NoError(t, engine.Start(t.Context()))
	defer engine.Stop()
	job, err := engine.ScheduleRecurring("alice", "title", "message", "@every 1h", nil)
	require.NoError(t, err)
	oldContext := engine.ctx
	engine.Stop()
	require.NoError(t, engine.Start(t.Context()))
	assert.Len(t, engine.cronRunner.Entries(), 1)
	engine.executeJob(job.ID, job.Revision, oldContext)
	assert.Zero(t, calls.Load())
	engine.executeJob(job.ID, job.Revision, engine.ctx)
	assert.Equal(t, int32(1), calls.Load())
}
