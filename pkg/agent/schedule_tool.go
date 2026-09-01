package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/scheduler"
)

type contextKey string

const (
	userIDContextKey contextKey = "user_id"
)

// WithUserID returns a child context with the specified UserID attached.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

// UserIDFromContext retrieves the UserID from the context.
func UserIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(userIDContextKey).(string); ok {
		return v
	}
	return ""
}

// ScheduleNotificationTool allows the AI Agent to register reminders and notifications.
type ScheduleNotificationTool struct {
	engine *scheduler.Engine
}

// NewScheduleNotificationTool creates a ScheduleNotificationTool.
func NewScheduleNotificationTool(engine *scheduler.Engine) *ScheduleNotificationTool {
	return &ScheduleNotificationTool{engine: engine}
}

func (t *ScheduleNotificationTool) Name() string {
	return "schedule_notification"
}

func (t *ScheduleNotificationTool) Description() string {
	return "Schedule a KakaoTalk notification or reminder at a specific time or on a recurring cron schedule. Supports one-shot reminders ('once') or recurring reminders ('recurring')."
}

func (t *ScheduleNotificationTool) ParametersSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Notification title (e.g. '정상어학원 2호차 출발 알림', '라면 끓이기 알림').",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Notification message body.",
			},
			"time_type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"once", "recurring"},
				"description": "Schedule type: 'once' for one-time reminder, 'recurring' for periodic reminder.",
			},
			"execute_at": map[string]interface{}{
				"type":        "string",
				"description": "Execution timestamp for 'once' type. Can be ISO 8601 (2026-09-01T15:00:00+09:00), 'YYYY-MM-DD HH:MM:SS', 'HH:MM', or relative like '+10m', '+1h'.",
			},
			"cron_expr": map[string]interface{}{
				"type":        "string",
				"description": "Standard 5 or 6-field Cron expression for 'recurring' type (e.g. '0 15 * * 1-5' for weekdays at 15:00, '@daily').",
			},
		},
		"required": []string{"title", "message", "time_type"},
	}
}

func (t *ScheduleNotificationTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context cancelled before schedule execution: %w", err)
	}
	if t.engine == nil {
		return "", fmt.Errorf("scheduler engine is not configured")
	}

	userID := UserIDFromContext(ctx)
	if userID == "" {
		userID = "kakao_user"
	}

	var args struct {
		Title     string `json:"title"`
		Message   string `json:"message"`
		TimeType  string `json:"time_type"`
		ExecuteAt string `json:"execute_at"`
		CronExpr  string `json:"cron_expr"`
	}

	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid args JSON: %w", err)
	}

	loc := t.engine.Location()
	if loc == nil {
		loc = time.Local
	}

	if args.TimeType == "recurring" || args.CronExpr != "" {
		cronExpr := strings.TrimSpace(args.CronExpr)
		if cronExpr == "" {
			return "", fmt.Errorf("cron_expr is required for recurring schedule")
		}
		job, err := t.engine.ScheduleRecurring(userID, args.Title, args.Message, cronExpr, nil)
		if err != nil {
			return fmt.Sprintf("Error creating recurring schedule: %v", err), nil
		}
		return fmt.Sprintf("✅ 반복 알림이 성공적으로 등록되었습니다.\n- 알림 ID: %s\n- 제목: %s\n- Cron 주기: %s\n- 메시지: %s",
			job.ID, job.Title, job.CronExpr, job.Message), nil
	}

	// Parse execute_at
	execTime, err := parseExecuteTime(args.ExecuteAt, loc)
	if err != nil {
		return fmt.Sprintf("Error parsing execution time %q: %v", args.ExecuteAt, err), nil
	}

	job, err := t.engine.ScheduleOnce(userID, args.Title, args.Message, execTime, nil)
	if err != nil {
		return fmt.Sprintf("Error creating schedule: %v", err), nil
	}

	return fmt.Sprintf("✅ 알림이 성공적으로 예약되었습니다.\n- 알림 ID: %s\n- 제목: %s\n- 예정 시각: %s (KST)\n- 메시지: %s",
		job.ID, job.Title, job.ExecuteAt.Format("2006-01-02 15:04:05"), job.Message), nil
}

// ListSchedulesTool allows the Agent to list registered schedules for the current user.
type ListSchedulesTool struct {
	engine *scheduler.Engine
}

func NewListSchedulesTool(engine *scheduler.Engine) *ListSchedulesTool {
	return &ListSchedulesTool{engine: engine}
}

func (t *ListSchedulesTool) Name() string {
	return "list_schedules"
}

func (t *ListSchedulesTool) Description() string {
	return "List all active, pending, or completed notification schedules for the current user."
}

func (t *ListSchedulesTool) ParametersSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *ListSchedulesTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	if t.engine == nil {
		return "", fmt.Errorf("scheduler engine is not configured")
	}

	userID := UserIDFromContext(ctx)
	jobs := t.engine.ListJobs(userID)
	if len(jobs) == 0 {
		return "현재 등록된 스케줄 및 알림이 없습니다.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 현재 등록된 스케줄 목록 (총 %d건):\n\n", len(jobs)))
	for i, j := range jobs {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, strings.ToUpper(string(j.Status)), j.Title))
		sb.WriteString(fmt.Sprintf("   - ID: %s\n", j.ID))
		if j.Type == scheduler.ScheduleTypeOnce {
			sb.WriteString(fmt.Sprintf("   - 실행 시각: %s\n", j.ExecuteAt.Format("2006-01-02 15:04:05")))
		} else {
			sb.WriteString(fmt.Sprintf("   - Cron 주기: %s\n", j.CronExpr))
		}
		sb.WriteString(fmt.Sprintf("   - 내용: %s\n", j.Message))
	}

	return strings.TrimSpace(sb.String()), nil
}

// CancelScheduleTool allows the Agent to cancel a scheduled notification owned by the current user.
type CancelScheduleTool struct {
	engine *scheduler.Engine
}

func NewCancelScheduleTool(engine *scheduler.Engine) *CancelScheduleTool {
	return &CancelScheduleTool{engine: engine}
}

func (t *CancelScheduleTool) Name() string {
	return "cancel_schedule"
}

func (t *CancelScheduleTool) Description() string {
	return "Cancel a scheduled notification by its Job ID."
}

func (t *CancelScheduleTool) ParametersSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"job_id": map[string]interface{}{
				"type":        "string",
				"description": "The Job ID of the schedule to cancel (e.g. 'job_178818...').",
			},
		},
		"required": []string{"job_id"},
	}
}

func (t *CancelScheduleTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context cancelled before cancel execution: %w", err)
	}
	if t.engine == nil {
		return "", fmt.Errorf("scheduler engine is not configured")
	}

	var args struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid args JSON: %w", err)
	}

	if strings.TrimSpace(args.JobID) == "" {
		return "", fmt.Errorf("job_id is required")
	}

	userID := UserIDFromContext(ctx)
	if err := t.engine.CancelJob(args.JobID, userID); err != nil {
		return fmt.Sprintf("Error cancelling schedule %s: %v", args.JobID, err), nil
	}

	return fmt.Sprintf("✅ 스케줄 ID %s 가 성공적으로 취소되었습니다.", args.JobID), nil
}

// UpdateScheduleTool allows the Agent to modify an existing schedule owned by the current user.
type UpdateScheduleTool struct {
	engine *scheduler.Engine
}

func NewUpdateScheduleTool(engine *scheduler.Engine) *UpdateScheduleTool {
	return &UpdateScheduleTool{engine: engine}
}

func (t *UpdateScheduleTool) Name() string {
	return "update_schedule"
}

func (t *UpdateScheduleTool) Description() string {
	return "Update an existing schedule's title, message, execution time, or cron expression by its Job ID."
}

func (t *UpdateScheduleTool) ParametersSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"job_id": map[string]interface{}{
				"type":        "string",
				"description": "The Job ID of the schedule to update.",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Optional new title.",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Optional new message body.",
			},
			"execute_at": map[string]interface{}{
				"type":        "string",
				"description": "Optional new execution timestamp (e.g. '+20m', '15:30', '2026-09-01 16:00:00').",
			},
			"cron_expr": map[string]interface{}{
				"type":        "string",
				"description": "Optional new cron expression (e.g. '0 16 * * 1-5').",
			},
		},
		"required": []string{"job_id"},
	}
}

func (t *UpdateScheduleTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context cancelled before update execution: %w", err)
	}
	if t.engine == nil {
		return "", fmt.Errorf("scheduler engine is not configured")
	}

	var args struct {
		JobID     string  `json:"job_id"`
		Title     *string `json:"title"`
		Message   *string `json:"message"`
		ExecuteAt *string `json:"execute_at"`
		CronExpr  *string `json:"cron_expr"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid args JSON: %w", err)
	}

	if strings.TrimSpace(args.JobID) == "" {
		return "", fmt.Errorf("job_id is required")
	}

	update := scheduler.JobUpdate{
		Title:   args.Title,
		Message: args.Message,
	}

	loc := t.engine.Location()
	if loc == nil {
		loc = time.Local
	}

	if args.ExecuteAt != nil && *args.ExecuteAt != "" {
		execTime, err := parseExecuteTime(*args.ExecuteAt, loc)
		if err != nil {
			return fmt.Sprintf("Error parsing new execution time: %v", err), nil
		}
		update.ExecuteAt = &execTime
	}

	if args.CronExpr != nil && *args.CronExpr != "" {
		update.CronExpr = args.CronExpr
	}

	userID := UserIDFromContext(ctx)
	job, err := t.engine.UpdateJob(args.JobID, update, userID)
	if err != nil {
		return fmt.Sprintf("Error updating schedule %s: %v", args.JobID, err), nil
	}

	return fmt.Sprintf("✅ 스케줄 ID %s 가 성공적으로 수정되었습니다.\n- 제목: %s\n- 메시지: %s", job.ID, job.Title, job.Message), nil
}

// DeleteScheduleTool allows the Agent to permanently delete a schedule owned by the current user.
type DeleteScheduleTool struct {
	engine *scheduler.Engine
}

func NewDeleteScheduleTool(engine *scheduler.Engine) *DeleteScheduleTool {
	return &DeleteScheduleTool{engine: engine}
}

func (t *DeleteScheduleTool) Name() string {
	return "delete_schedule"
}

func (t *DeleteScheduleTool) Description() string {
	return "Permanently delete a scheduled notification by its Job ID."
}

func (t *DeleteScheduleTool) ParametersSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"job_id": map[string]interface{}{
				"type":        "string",
				"description": "The Job ID of the schedule to permanently delete.",
			},
		},
		"required": []string{"job_id"},
	}
}

func (t *DeleteScheduleTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context cancelled before delete execution: %w", err)
	}
	if t.engine == nil {
		return "", fmt.Errorf("scheduler engine is not configured")
	}

	var args struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid args JSON: %w", err)
	}

	if strings.TrimSpace(args.JobID) == "" {
		return "", fmt.Errorf("job_id is required")
	}

	userID := UserIDFromContext(ctx)
	if err := t.engine.DeleteJob(args.JobID, userID); err != nil {
		return fmt.Sprintf("Error deleting schedule %s: %v", args.JobID, err), nil
	}

	return fmt.Sprintf("✅ 스케줄 ID %s 가 영구 삭제되었습니다.", args.JobID), nil
}

func parseExecuteTime(input string, loc *time.Location) (time.Time, error) {
	input = strings.TrimSpace(input)
	now := time.Now().In(loc)

	// Relative format like "+10m", "+1h", "10m", "30s"
	if strings.HasPrefix(input, "+") || strings.HasSuffix(input, "m") || strings.HasSuffix(input, "s") || strings.HasSuffix(input, "h") {
		clean := strings.TrimPrefix(input, "+")
		dur, err := time.ParseDuration(clean)
		if err == nil {
			return now.Add(dur), nil
		}
	}

	// Relative "X분 뒤", "X시간 뒤"
	if strings.Contains(input, "분") || strings.Contains(input, "시간") || strings.Contains(input, "초") {
		var totalDur time.Duration
		parts := strings.Fields(input)
		for _, p := range parts {
			if strings.HasSuffix(p, "초") || strings.HasSuffix(p, "초뒤") || strings.HasSuffix(p, "초후") {
				numStr := strings.TrimRight(p, "초뒤후 ")
				if n, err := strconv.Atoi(numStr); err == nil {
					totalDur += time.Duration(n) * time.Second
				}
			} else if strings.HasSuffix(p, "분") || strings.HasSuffix(p, "분뒤") || strings.HasSuffix(p, "분후") {
				numStr := strings.TrimRight(p, "분뒤후 ")
				if n, err := strconv.Atoi(numStr); err == nil {
					totalDur += time.Duration(n) * time.Minute
				}
			} else if strings.HasSuffix(p, "시간") || strings.HasSuffix(p, "시간뒤") || strings.HasSuffix(p, "시간후") {
				numStr := strings.TrimRight(p, "시간뒤후 ")
				if n, err := strconv.Atoi(numStr); err == nil {
					totalDur += time.Duration(n) * time.Hour
				}
			}
		}
		if totalDur > 0 {
			return now.Add(totalDur), nil
		}
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"15:04:05",
		"15:04",
	}

	for _, f := range formats {
		t, err := time.ParseInLocation(f, input, loc)
		if err == nil {
			// If only time was given (e.g. 15:04), combine with today's date
			if f == "15:04:05" || f == "15:04" {
				t = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, loc)
				if t.Before(now) {
					// Schedule for tomorrow if time already passed today
					t = t.AddDate(0, 0, 1)
				}
			}
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized time format %q", input)
}
