package scheduler

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// EngineOption configures the scheduler Engine.
type EngineOption func(*Engine)

// WithLocation sets the timezone for the engine.
func WithLocation(loc *time.Location) EngineOption {
	return func(e *Engine) {
		e.loc = loc
	}
}

// Engine coordinates job scheduling, persistence, and execution.
type Engine struct {
	store      Store
	handler    Handler
	loc        *time.Location
	cronRunner *cron.Cron
	entryMap   map[string]cron.EntryID
	timerMap   map[string]*time.Timer
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
	running    bool
}

// NewEngine creates a new scheduler Engine.
func NewEngine(store Store, handler Handler, opts ...EngineOption) *Engine {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		loc = time.FixedZone("KST", 9*60*60)
	}

	e := &Engine{
		store:    store,
		handler:  handler,
		loc:      loc,
		entryMap: make(map[string]cron.EntryID),
		timerMap: make(map[string]*time.Timer),
	}

	for _, opt := range opts {
		opt(e)
	}

	e.cronRunner = cron.New(
		cron.WithLocation(e.loc),
		cron.WithSeconds(),
		cron.WithParser(cron.NewParser(
			cron.SecondOptional|cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow|cron.Descriptor,
		)),
		cron.WithChain(
			cron.Recover(cron.DefaultLogger),
			cron.SkipIfStillRunning(cron.DefaultLogger),
		),
	)

	return e
}

// Location returns the configured timezone.
func (e *Engine) Location() *time.Location {
	return e.loc
}

// Start boots the scheduler engine and restores active jobs.
func (e *Engine) Start(parentCtx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	if parentCtx == nil {
		parentCtx = context.Background()
	}
	e.ctx, e.cancel = context.WithCancel(parentCtx)
	e.cronRunner.Start()
	e.running = true

	// Auto-prune jobs completed or cancelled more than 7 days ago
	e.purgeCompletedJobsLocked(7 * 24 * time.Hour)

	// Restore active jobs from store
	jobs, err := e.store.List()
	if err != nil {
		return fmt.Errorf("list store jobs: %w", err)
	}

	now := time.Now().In(e.loc)
	for _, job := range jobs {
		if job.Status != JobStatusActive {
			continue
		}

		if job.Type == ScheduleTypeOnce {
			if job.ExecuteAt.Before(now) {
				// Mark missed past jobs as completed
				job.Status = JobStatusCompleted
				_ = e.store.Save(job)
				continue
			}
			e.scheduleTimerLocked(job)
		} else if job.Type == ScheduleTypeRecurring {
			_ = e.scheduleCronLocked(job)
		}
	}

	return nil
}

// Stop terminates the scheduler engine.
func (e *Engine) Stop() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}

	if e.cancel != nil {
		e.cancel()
	}

	for _, timer := range e.timerMap {
		timer.Stop()
	}
	e.timerMap = make(map[string]*time.Timer)
	e.entryMap = make(map[string]cron.EntryID)
	e.running = false
	cronRunner := e.cronRunner
	e.mu.Unlock()

	// Wait for cron stop outside the mutex lock to prevent deadlocks
	if cronRunner != nil {
		ctx := cronRunner.Stop()
		<-ctx.Done()
	}
}

// ScheduleOnce creates and schedules a one-time notification.
func (e *Engine) ScheduleOnce(userID, title, message string, executeAt time.Time, payload map[string]string) (*Job, error) {
	now := time.Now().In(e.loc)
	executeAt = executeAt.In(e.loc)

	if executeAt.Before(now) {
		return nil, fmt.Errorf("execute time %s is in the past", executeAt.Format(time.RFC3339))
	}

	job := &Job{
		ID:        fmt.Sprintf("job_%d", time.Now().UnixNano()),
		UserID:    userID,
		Title:     title,
		Message:   message,
		Type:      ScheduleTypeOnce,
		ExecuteAt: executeAt,
		CreatedAt: now,
		Status:    JobStatusActive,
		Payload:   payload,
	}

	if err := e.store.Save(job); err != nil {
		return nil, fmt.Errorf("save job: %w", err)
	}

	e.mu.Lock()
	if e.running {
		e.scheduleTimerLocked(job)
	}
	e.mu.Unlock()

	return job.Clone(), nil
}

// ScheduleRecurring creates and schedules a recurring cron notification.
func (e *Engine) ScheduleRecurring(userID, title, message string, cronExpr string, payload map[string]string) (*Job, error) {
	now := time.Now().In(e.loc)

	job := &Job{
		ID:        fmt.Sprintf("job_%d", time.Now().UnixNano()),
		UserID:    userID,
		Title:     title,
		Message:   message,
		Type:      ScheduleTypeRecurring,
		CronExpr:  cronExpr,
		CreatedAt: now,
		Status:    JobStatusActive,
		Payload:   payload,
	}

	if err := e.store.Save(job); err != nil {
		return nil, fmt.Errorf("save job: %w", err)
	}

	e.mu.Lock()
	if e.running {
		if err := e.scheduleCronLocked(job); err != nil {
			e.mu.Unlock()
			job.Status = JobStatusFailed
			_ = e.store.Save(job)
			return nil, err
		}
	}
	e.mu.Unlock()

	return job.Clone(), nil
}

func (e *Engine) scheduleTimerLocked(job *Job) {
	delay := time.Until(job.ExecuteAt)
	if delay < 0 {
		delay = 0
	}

	jobID := job.ID
	timer := time.AfterFunc(delay, func() {
		e.executeJob(jobID)
	})

	e.timerMap[jobID] = timer
}

func (e *Engine) scheduleCronLocked(job *Job) error {
	jobID := job.ID
	entryID, err := e.cronRunner.AddFunc(job.CronExpr, func() {
		e.executeJob(jobID)
	})
	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", job.CronExpr, err)
	}

	e.entryMap[jobID] = entryID
	return nil
}

func (e *Engine) executeJob(jobID string) {
	e.mu.Lock()
	if timer, ok := e.timerMap[jobID]; ok {
		timer.Stop()
		delete(e.timerMap, jobID)
	}
	e.mu.Unlock()

	job, err := e.store.Get(jobID)
	if err != nil || job.Status != JobStatusActive {
		return
	}

	now := time.Now().In(e.loc)
	job.LastFiredAt = &now

	if job.Type == ScheduleTypeOnce {
		job.Status = JobStatusCompleted
	}

	_ = e.store.Save(job)

	if e.handler != nil {
		e.mu.RLock()
		ctx := e.ctx
		e.mu.RUnlock()

		if ctx == nil {
			ctx = context.Background()
		}
		if err := e.handler(ctx, job); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ [Scheduler] Job %s handler error: %v\n", jobID, err)
		}
	}
}

// CancelJob cancels a scheduled job. If ownerUserID is provided, it verifies ownership.
func (e *Engine) CancelJob(id string, ownerUserID ...string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	job, err := e.store.Get(id)
	if err != nil {
		return err
	}

	if len(ownerUserID) > 0 && ownerUserID[0] != "" {
		if job.UserID != "" && !strings.EqualFold(job.UserID, ownerUserID[0]) {
			return fmt.Errorf("permission denied: job %s belongs to another user", id)
		}
	}

	job.Status = JobStatusCancelled
	_ = e.store.Save(job)

	if timer, ok := e.timerMap[id]; ok {
		timer.Stop()
		delete(e.timerMap, id)
	}

	if entryID, ok := e.entryMap[id]; ok {
		e.cronRunner.Remove(entryID)
		delete(e.entryMap, id)
	}

	return nil
}

// UpdateJob updates an existing job and re-arms its timer/cron runner. If ownerUserID is provided, it verifies ownership.
func (e *Engine) UpdateJob(id string, update JobUpdate, ownerUserID ...string) (*Job, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	job, err := e.store.Get(id)
	if err != nil {
		return nil, err
	}

	if len(ownerUserID) > 0 && ownerUserID[0] != "" {
		if job.UserID != "" && !strings.EqualFold(job.UserID, ownerUserID[0]) {
			return nil, fmt.Errorf("permission denied: job %s belongs to another user", id)
		}
	}

	// Stop and remove existing schedule triggers
	if timer, ok := e.timerMap[id]; ok {
		timer.Stop()
		delete(e.timerMap, id)
	}
	if entryID, ok := e.entryMap[id]; ok {
		e.cronRunner.Remove(entryID)
		delete(e.entryMap, id)
	}

	// Apply updates
	if update.Title != nil {
		job.Title = *update.Title
	}
	if update.Message != nil {
		job.Message = *update.Message
	}
	if update.ExecuteAt != nil {
		job.ExecuteAt = *update.ExecuteAt
		job.Type = ScheduleTypeOnce
		job.CronExpr = ""
	}
	if update.CronExpr != nil {
		job.CronExpr = *update.CronExpr
		job.Type = ScheduleTypeRecurring
	}
	if update.Status != nil {
		job.Status = *update.Status
	}

	// Re-arm schedule if active
	if job.Status == JobStatusActive && e.running {
		if job.Type == ScheduleTypeOnce {
			if job.ExecuteAt.After(time.Now().In(e.loc)) {
				e.scheduleTimerLocked(job)
			} else {
				job.Status = JobStatusCompleted
			}
		} else if job.Type == ScheduleTypeRecurring {
			if err := e.scheduleCronLocked(job); err != nil {
				return nil, err
			}
		}
	}

	if err := e.store.Save(job); err != nil {
		return nil, err
	}
	return job.Clone(), nil
}

// DeleteJob permanently removes a job from scheduler and store. If ownerUserID is provided, it verifies ownership.
func (e *Engine) DeleteJob(id string, ownerUserID ...string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	job, err := e.store.Get(id)
	if err != nil {
		return err
	}

	if len(ownerUserID) > 0 && ownerUserID[0] != "" {
		if job.UserID != "" && !strings.EqualFold(job.UserID, ownerUserID[0]) {
			return fmt.Errorf("permission denied: job %s belongs to another user", id)
		}
	}

	if timer, ok := e.timerMap[id]; ok {
		timer.Stop()
		delete(e.timerMap, id)
	}
	if entryID, ok := e.entryMap[id]; ok {
		e.cronRunner.Remove(entryID)
		delete(e.entryMap, id)
	}

	return e.store.Delete(id)
}

// ListJobs returns jobs filtered by user ID. If userID is empty or "*", it returns all jobs.
func (e *Engine) ListJobs(userID string) []*Job {
	jobs, err := e.store.List()
	if err != nil {
		return nil
	}

	if userID == "" || userID == "*" {
		list := make([]*Job, 0, len(jobs))
		for _, j := range jobs {
			list = append(list, j.Clone())
		}
		return list
	}

	filtered := make([]*Job, 0)
	for _, j := range jobs {
		if strings.EqualFold(j.UserID, userID) {
			filtered = append(filtered, j.Clone())
		}
	}
	return filtered
}

// ListAllJobs returns all jobs without user filter (for administrative / internal inspection).
func (e *Engine) ListAllJobs() []*Job {
	jobs, err := e.store.List()
	if err != nil {
		return nil
	}
	list := make([]*Job, 0, len(jobs))
	for _, j := range jobs {
		list = append(list, j.Clone())
	}
	return list
}

// GetJob returns a job by ID.
func (e *Engine) GetJob(id string) (*Job, error) {
	return e.store.Get(id)
}

// PurgeCompletedJobs removes completed, cancelled, or failed jobs older than maxAge.
func (e *Engine) PurgeCompletedJobs(maxAge time.Duration) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.purgeCompletedJobsLocked(maxAge)
}

func (e *Engine) purgeCompletedJobsLocked(maxAge time.Duration) int {
	jobs, err := e.store.List()
	if err != nil {
		return 0
	}

	now := time.Now().In(e.loc)
	purged := 0
	for _, j := range jobs {
		if j.Status == JobStatusCompleted || j.Status == JobStatusCancelled || j.Status == JobStatusFailed {
			refTime := j.CreatedAt
			if j.LastFiredAt != nil {
				refTime = *j.LastFiredAt
			}

			if now.Sub(refTime) > maxAge {
				if err := e.store.Delete(j.ID); err == nil {
					purged++
				}
			}
		}
	}
	return purged
}
