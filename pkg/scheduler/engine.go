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
	// Auto-prune jobs completed or cancelled more than 7 days ago
	e.purgeCompletedJobsLocked(7 * 24 * time.Hour)

	// Restore active jobs from store
	jobs, err := e.store.List()
	if err != nil {
		return fmt.Errorf("list store jobs: %w", err)
	}

	now := time.Now().In(e.loc)
	schedules := make(map[string]cron.Schedule)
	for _, job := range jobs {
		if job.Status != JobStatusActive {
			continue
		}

		if job.Type == ScheduleTypeOnce {
			if job.ExecuteAt.Before(now) {
				// Mark missed past jobs as completed
				job.Status = JobStatusCompleted
				if err := e.store.Save(job); err != nil {
					return fmt.Errorf("complete missed job %s: %w", job.ID, err)
				}
				continue
			}
		} else if job.Type == ScheduleTypeRecurring {
			schedule, err := parseCron(job.CronExpr)
			if err != nil {
				return fmt.Errorf("restore job %s: %w", job.ID, err)
			}
			schedules[job.ID] = schedule
		}
	}
	e.ctx, e.cancel = context.WithCancel(parentCtx)
	e.running = true
	for _, job := range jobs {
		if job.Status != JobStatusActive {
			continue
		}
		if job.Type == ScheduleTypeOnce {
			e.scheduleTimerLocked(job)
		} else if job.Type == ScheduleTypeRecurring {
			e.scheduleCronLocked(job, schedules[job.ID])
		}
	}
	e.cronRunner.Start()

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
	for _, entryID := range e.entryMap {
		e.cronRunner.Remove(entryID)
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
		Revision:  1,
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.store.Save(job); err != nil {
		return nil, fmt.Errorf("save job: %w", err)
	}

	if e.running {
		e.scheduleTimerLocked(job)
	}

	return job.Clone(), nil
}

// ScheduleRecurring creates and schedules a recurring cron notification.
func (e *Engine) ScheduleRecurring(userID, title, message string, cronExpr string, payload map[string]string) (*Job, error) {
	schedule, err := parseCron(cronExpr)
	if err != nil {
		return nil, err
	}
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
		Revision:  1,
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.store.Save(job); err != nil {
		return nil, fmt.Errorf("save job: %w", err)
	}

	if e.running {
		e.scheduleCronLocked(job, schedule)
	}

	return job.Clone(), nil
}

func (e *Engine) scheduleTimerLocked(job *Job) {
	delay := time.Until(job.ExecuteAt)
	if delay < 0 {
		delay = 0
	}

	jobID := job.ID
	revision, runCtx := job.Revision, e.ctx
	timer := time.AfterFunc(delay, func() {
		e.executeJob(jobID, revision, runCtx)
	})

	e.timerMap[jobID] = timer
}

func parseCron(expr string) (cron.Schedule, error) {
	schedule, err := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor).Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression %q: %w", expr, err)
	}
	return schedule, nil
}

func (e *Engine) scheduleCronLocked(job *Job, schedule cron.Schedule) {
	jobID, revision, runCtx := job.ID, job.Revision, e.ctx
	e.entryMap[jobID] = e.cronRunner.Schedule(schedule, cron.FuncJob(func() {
		e.executeJob(jobID, revision, runCtx)
	}))
}

func (e *Engine) executeJob(jobID string, revision uint64, runCtx context.Context) {
	e.mu.Lock()
	if !e.running || e.ctx != runCtx {
		e.mu.Unlock()
		return
	}
	job, err := e.store.Get(jobID)
	if err != nil || job.Status != JobStatusActive || job.Revision != revision {
		e.mu.Unlock()
		return
	}
	if timer, ok := e.timerMap[jobID]; ok {
		timer.Stop()
		delete(e.timerMap, jobID)
	}
	e.mu.Unlock()

	now := time.Now().In(e.loc)
	job.LastFiredAt = &now

	var handlerErr error
	if e.handler != nil {
		if err := e.handler(runCtx, job); err != nil {
			handlerErr = err
			fmt.Fprintf(os.Stderr, "⚠️ [Scheduler] Job %s handler error: %v\n", jobID, err)
		}
	}

	// Merge execution metadata into the latest committed job. A deletion,
	// cancellation or reschedule wins over this in-flight delivery.
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running || e.ctx != runCtx {
		return
	}
	current, err := e.store.Get(jobID)
	if err != nil || current.Status != JobStatusActive || current.Revision != revision {
		return
	}
	current.LastFiredAt = &now
	if current.Type == ScheduleTypeOnce {
		if handlerErr == nil {
			current.Status = JobStatusCompleted
		} else {
			current.Status = JobStatusFailed
		}
	}

	if err := e.store.Save(current); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ [Scheduler] Failed to persist execution of job %s: %v\n", jobID, err)
	}
}

// authorizeOwner enforces per-user isolation on job mutations.
// With no ownerUserID argument the call is administrative and unrestricted.
// With one, a nonempty job UserID must equal the caller exactly. Legacy ownerless
// jobs are accessible only through the explicit administrative path.
func authorizeOwner(job *Job, id string, ownerUserID []string) error {
	if len(ownerUserID) == 0 {
		return nil
	}
	requiredUser := ownerUserID[0]
	if requiredUser != "" && requiredUser != "*" && requiredUser == strings.TrimSpace(requiredUser) && job.UserID == requiredUser {
		return nil
	}
	return fmt.Errorf("permission denied: job %s belongs to another user", id)
}

// CancelJob cancels a scheduled job. If ownerUserID is provided, it verifies ownership.
func (e *Engine) CancelJob(id string, ownerUserID ...string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	job, err := e.store.Get(id)
	if err != nil {
		return err
	}

	if err := authorizeOwner(job, id, ownerUserID); err != nil {
		return err
	}

	job.Status = JobStatusCancelled
	job.Revision++
	if err := e.store.Save(job); err != nil {
		return fmt.Errorf("persist cancellation: %w", err)
	}

	e.removeTriggerLocked(id)

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

	if err := authorizeOwner(job, id, ownerUserID); err != nil {
		return nil, err
	}

	if update.ExecuteAt != nil && update.CronExpr != nil {
		return nil, fmt.Errorf("execute_at and cron_expr cannot both be updated")
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
		job.ExecuteAt = time.Time{}
	}
	if update.Status != nil {
		job.Status = *update.Status
	}

	reschedule := update.ExecuteAt != nil || update.CronExpr != nil || update.Status != nil
	var schedule cron.Schedule
	if job.Type == ScheduleTypeRecurring {
		schedule, err = parseCron(job.CronExpr)
		if err != nil {
			return nil, err
		}
	}
	if reschedule && job.Status == JobStatusActive && job.Type == ScheduleTypeOnce && !job.ExecuteAt.After(time.Now().In(e.loc)) {
		return nil, fmt.Errorf("execute time must be in the future")
	}
	switch job.Status {
	case JobStatusActive, JobStatusCompleted, JobStatusCancelled, JobStatusFailed:
	default:
		return nil, fmt.Errorf("invalid job status %q", job.Status)
	}
	if reschedule {
		job.Revision++
	}

	if err := e.store.Save(job); err != nil {
		return nil, err
	}
	// Only replace triggers after validation and persistence succeed. Metadata
	// edits preserve the current trigger and may be merged by an in-flight run.
	if reschedule {
		e.removeTriggerLocked(id)
		if e.running && job.Status == JobStatusActive {
			if job.Type == ScheduleTypeOnce {
				e.scheduleTimerLocked(job)
			} else if job.Type == ScheduleTypeRecurring {
				e.scheduleCronLocked(job, schedule)
			}
		}
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

	if err := authorizeOwner(job, id, ownerUserID); err != nil {
		return err
	}

	if err := e.store.Delete(id); err != nil {
		return err
	}
	e.removeTriggerLocked(id)
	return nil
}

func (e *Engine) removeTriggerLocked(id string) {
	if timer, ok := e.timerMap[id]; ok {
		timer.Stop()
		delete(e.timerMap, id)
	}
	if entryID, ok := e.entryMap[id]; ok {
		e.cronRunner.Remove(entryID)
		delete(e.entryMap, id)
	}
}

// ListJobs returns only jobs owned by the exact user ID. Administrative callers
// must use ListAllJobs; empty, whitespace and wildcard IDs cannot widen access.
func (e *Engine) ListJobs(userID string) []*Job {
	if userID == "" || userID == "*" || userID != strings.TrimSpace(userID) {
		return nil
	}
	jobs, err := e.store.List()
	if err != nil {
		return nil
	}

	filtered := make([]*Job, 0)
	for _, j := range jobs {
		if j.UserID == userID {
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
