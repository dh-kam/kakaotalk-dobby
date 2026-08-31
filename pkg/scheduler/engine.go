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
	if e.running {
		e.mu.Unlock()
		return nil
	}

	e.ctx, e.cancel = context.WithCancel(parentCtx)
	e.cronRunner.Start()
	e.running = true
	e.mu.Unlock()

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
				// Mark missed jobs as completed or missed
				job.Status = JobStatusCompleted
				_ = e.store.Save(job)
				continue
			}
			e.scheduleTimer(job)
		} else if job.Type == ScheduleTypeRecurring {
			_ = e.scheduleCron(job)
		}
	}

	return nil
}

// Stop terminates the scheduler engine.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	if e.cancel != nil {
		e.cancel()
	}
	ctx := e.cronRunner.Stop()
	<-ctx.Done()

	for _, timer := range e.timerMap {
		timer.Stop()
	}
	e.timerMap = make(map[string]*time.Timer)
	e.entryMap = make(map[string]cron.EntryID)
	e.running = false
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
		e.scheduleTimer(job)
	}
	e.mu.Unlock()

	return job, nil
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
		if err := e.scheduleCron(job); err != nil {
			e.mu.Unlock()
			job.Status = JobStatusFailed
			_ = e.store.Save(job)
			return nil, err
		}
	}
	e.mu.Unlock()

	return job, nil
}

func (e *Engine) scheduleTimer(job *Job) {
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

func (e *Engine) scheduleCron(job *Job) error {
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
	job, err := e.store.Get(jobID)
	if err != nil || job.Status != JobStatusActive {
		return
	}

	now := time.Now().In(e.loc)
	job.LastFiredAt = &now

	if job.Type == ScheduleTypeOnce {
		job.Status = JobStatusCompleted
		e.mu.Lock()
		delete(e.timerMap, jobID)
		e.mu.Unlock()
	}

	_ = e.store.Save(job)

	if e.handler != nil {
		ctx := e.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		if err := e.handler(ctx, job); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ [Scheduler] Job %s handler error: %v\n", jobID, err)
		}
	}
}

// CancelJob cancels a scheduled job.
func (e *Engine) CancelJob(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	job, err := e.store.Get(id)
	if err != nil {
		return err
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

// UpdateJob updates an existing job and re-arms its timer/cron runner.
func (e *Engine) UpdateJob(id string, update JobUpdate) (*Job, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	job, err := e.store.Get(id)
	if err != nil {
		return nil, err
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
				e.scheduleTimer(job)
			} else {
				job.Status = JobStatusCompleted
			}
		} else if job.Type == ScheduleTypeRecurring {
			if err := e.scheduleCron(job); err != nil {
				return nil, err
			}
		}
	}

	if err := e.store.Save(job); err != nil {
		return nil, err
	}
	return job, nil
}

// DeleteJob permanently removes a job from scheduler and store.
func (e *Engine) DeleteJob(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

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

// ListJobs returns all jobs, optionally filtered by user ID.
func (e *Engine) ListJobs(userID string) []*Job {
	jobs, err := e.store.List()
	if err != nil {
		return nil
	}

	if userID == "" {
		return jobs
	}

	filtered := make([]*Job, 0)
	for _, j := range jobs {
		if j.UserID == "" || strings.EqualFold(j.UserID, userID) {
			filtered = append(filtered, j)
		}
	}
	return filtered
}

// GetJob returns a job by ID.
func (e *Engine) GetJob(id string) (*Job, error) {
	return e.store.Get(id)
}

