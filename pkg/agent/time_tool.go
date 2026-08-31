package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// CurrentTimeTool returns current date and time in KST (Asia/Seoul, UTC+9).
type CurrentTimeTool struct{}

func (t *CurrentTimeTool) Name() string {
	return "get_current_time"
}

func (t *CurrentTimeTool) Description() string {
	return "Get the current date, time, day of week, and timezone in Korea Standard Time (KST, Asia/Seoul, UTC+9). Use this tool to get the current reference time for bus schedules, relative time calculations, or setting reminders."
}

func (t *CurrentTimeTool) ParametersSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *CurrentTimeTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		loc = time.FixedZone("KST", 9*60*60)
	}
	now := time.Now().In(loc)

	weekdaysKo := []string{"일요일", "월요일", "화요일", "수요일", "목요일", "금요일", "토요일"}
	weekdayKo := weekdaysKo[now.Weekday()]

	result := map[string]interface{}{
		"iso8601":          now.Format(time.RFC3339),
		"formatted":        now.Format("2006-01-02 15:04:05 MST"),
		"korean_formatted": fmt.Sprintf("%d년 %d월 %d일 %s %02d:%02d:%02d (KST)", now.Year(), now.Month(), now.Day(), weekdayKo, now.Hour(), now.Minute(), now.Second()),
		"date":             now.Format("2006-01-02"),
		"time":             now.Format("15:04"),
		"hour":             now.Hour(),
		"minute":           now.Minute(),
		"weekday":          weekdayKo,
		"timezone":         "Asia/Seoul (KST, UTC+9)",
	}

	resBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal time result: %w", err)
	}

	return string(resBytes), nil
}
