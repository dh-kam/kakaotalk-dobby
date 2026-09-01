package scheduler

import (
	"context"
	"time"
)

// ScheduleType represents whether the job is one-shot or recurring.
type ScheduleType string

const (
	ScheduleTypeOnce      ScheduleType = "once"
	ScheduleTypeRecurring ScheduleType = "recurring"
)

// JobStatus represents the lifecycle state of a scheduled job.
type JobStatus string

const (
	JobStatusActive    JobStatus = "active"
	JobStatusCompleted JobStatus = "completed"
	JobStatusCancelled JobStatus = "cancelled"
	JobStatusFailed    JobStatus = "failed"
)

// Job represents a single scheduled notification task.
type Job struct {
	ID          string            `json:"id"`
	UserID      string            `json:"user_id,omitempty"`
	Title       string            `json:"title"`
	Message     string            `json:"message"`
	Type        ScheduleType      `json:"type"`
	ExecuteAt   time.Time         `json:"execute_at,omitempty"`
	CronExpr    string            `json:"cron_expr,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	LastFiredAt *time.Time        `json:"last_fired_at,omitempty"`
	Status      JobStatus         `json:"status"`
	Payload     map[string]string `json:"payload,omitempty"`
}

// Clone returns a deep copy of Job.
func (j *Job) Clone() *Job {
	if j == nil {
		return nil
	}
	cp := *j
	if j.LastFiredAt != nil {
		t := *j.LastFiredAt
		cp.LastFiredAt = &t
	}
	if j.Payload != nil {
		cp.Payload = make(map[string]string, len(j.Payload))
		for k, v := range j.Payload {
			cp.Payload[k] = v
		}
	}
	return &cp
}

// JobUpdate specifies fields to modify on an existing job.
type JobUpdate struct {
	Title     *string
	Message   *string
	ExecuteAt *time.Time
	CronExpr  *string
	Status    *JobStatus
}

// Handler handles the execution of a scheduled job (e.g. sending a Kakao notification).
type Handler func(ctx context.Context, job *Job) error
