package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/holidays"
)

// CurrentTimeTool returns current date, time, weekday, and holiday status in KST (Asia/Seoul, UTC+9).
type CurrentTimeTool struct{}

func (t *CurrentTimeTool) Name() string {
	return "get_current_time"
}

func (t *CurrentTimeTool) Description() string {
	return "Get current date, time, weekday, and Korean holiday/business day status in Korea Standard Time (KST, Asia/Seoul, UTC+9). Use this to get current reference time, check if today is a holiday or weekend, and perform date/time calculations."
}

func (t *CurrentTimeTool) ParametersSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *CurrentTimeTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	now := time.Now().In(holidays.GetKSTLocation())
	hInfo := holidays.CheckDate(now)

	statusStr := "평일 (영업일)"
	if hInfo.IsHoliday {
		statusStr = fmt.Sprintf("공휴일: %s", hInfo.HolidayName)
	} else if hInfo.IsWeekend {
		statusStr = fmt.Sprintf("주말 (%s)", hInfo.Weekday)
	}

	msg := fmt.Sprintf("현재 대한민국 표준시(KST)는 %d년 %d월 %d일 %s %02d:%02d:%02d (%s)입니다.", now.Year(), now.Month(), now.Day(), hInfo.Weekday, now.Hour(), now.Minute(), now.Second(), statusStr)

	result := map[string]interface{}{
		"message":          msg,
		"summary":          msg,
		"iso8601":          now.Format(time.RFC3339),
		"formatted":        now.Format("2006-01-02 15:04:05 MST"),
		"korean_formatted": fmt.Sprintf("%d년 %d월 %d일 %s %02d:%02d:%02d (%s)", now.Year(), now.Month(), now.Day(), hInfo.Weekday, now.Hour(), now.Minute(), now.Second(), statusStr),
		"date":             hInfo.Date,
		"time":             now.Format("15:04"),
		"year":             hInfo.Year,
		"month":            hInfo.Month,
		"day":              hInfo.Day,
		"hour":             now.Hour(),
		"minute":           now.Minute(),
		"weekday":          hInfo.Weekday,
		"is_holiday":       hInfo.IsHoliday,
		"is_weekend":       hInfo.IsWeekend,
		"is_business_day":  hInfo.IsBusinessDay,
		"holiday_name":     hInfo.HolidayName,
		"day_type":         statusStr,
		"timezone":         "Asia/Seoul (KST, UTC+9)",
	}

	resBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal time result: %w", err)
	}

	return string(resBytes), nil
}
