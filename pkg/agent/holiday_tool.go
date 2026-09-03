package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/holidays"
)

// KoreanHolidayTool evaluates whether a date is a Korean public holiday, weekend, or business day.
type KoreanHolidayTool struct{}

func (t *KoreanHolidayTool) Name() string {
	return "check_korean_holiday"
}

func (t *KoreanHolidayTool) Description() string {
	return "Check if a specified date (or today/tomorrow) is a Korean public holiday, substitute holiday, weekend, or business day according to the official Korean calendar (관공서의 공휴일). Can also return upcoming Korean holidays."
}

func (t *KoreanHolidayTool) ParametersSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"date": map[string]interface{}{
				"type":        "string",
				"description": "Optional date to check (e.g. '2026-09-25', '2026-10-09', '오늘', '내일', '모레'). Defaults to today in KST.",
			},
			"list_upcoming": map[string]interface{}{
				"type":        "boolean",
				"description": "Optional. Set to true to include upcoming Korean public holidays.",
			},
		},
	}
}

func (t *KoreanHolidayTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Date         string `json:"date"`
		ListUpcoming bool   `json:"list_upcoming"`
	}

	if argsJSON != "" && argsJSON != "{}" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid json arguments: %w", err)
		}
	}

	info, err := holidays.ParseAndCheck(args.Date)
	if err != nil {
		return fmt.Sprintf("날짜 파싱 실패: %v", err), nil
	}

	var statusText string
	if info.IsHoliday {
		subText := ""
		if info.IsSubstituteHoliday {
			subText = " (대체공휴일)"
		}
		statusText = fmt.Sprintf("🎉 %s (%s)은 대한민국 공휴일[%s%s]입니다.", info.Date, info.Weekday, info.HolidayName, subText)
	} else if info.IsWeekend {
		statusText = fmt.Sprintf("🏖️ %s (%s)은 주말(휴일)입니다.", info.Date, info.Weekday)
	} else {
		statusText = fmt.Sprintf("💼 %s (%s)은 정상 평일(영업일)입니다.", info.Date, info.Weekday)
	}

	response := map[string]interface{}{
		"message":               statusText,
		"summary":               statusText,
		"date":                  info.Date,
		"year":                  info.Year,
		"month":                 info.Month,
		"day":                   info.Day,
		"weekday":               info.Weekday,
		"is_holiday":            info.IsHoliday,
		"is_weekend":            info.IsWeekend,
		"is_business_day":       info.IsBusinessDay,
		"holiday_name":          info.HolidayName,
		"is_substitute_holiday": info.IsSubstituteHoliday,
		"description":           info.Description,
	}

	if args.ListUpcoming {
		fromTime, _ := time.ParseInLocation("2006-01-02", info.Date, holidays.GetKSTLocation())
		upcoming := holidays.GetUpcomingHolidays(fromTime, 5)
		var upcomingList []map[string]string
		for _, u := range upcoming {
			upcomingList = append(upcomingList, map[string]string{
				"date":         u.Date,
				"weekday":      u.Weekday,
				"holiday_name": u.HolidayName,
			})
		}
		response["upcoming_holidays"] = upcomingList
	}

	resBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal holiday result: %w", err)
	}

	return string(resBytes), nil
}
