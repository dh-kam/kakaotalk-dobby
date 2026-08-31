package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/holidays"
)

// BuildDynamicSystemPrompt injects live KST time, date, day of week, and holiday calendar context into the system prompt.
func BuildDynamicSystemPrompt(basePrompt string) string {
	now := time.Now().In(holidays.GetKSTLocation())
	hInfo := holidays.CheckDate(now)

	statusStr := "평일 (영업일)"
	if hInfo.IsHoliday {
		sub := ""
		if hInfo.IsSubstituteHoliday {
			sub = " (대체공휴일)"
		}
		statusStr = fmt.Sprintf("공휴일 [%s%s]", hInfo.HolidayName, sub)
	} else if hInfo.IsWeekend {
		statusStr = fmt.Sprintf("주말 (%s, 휴일)", hInfo.Weekday)
	}

	var sb strings.Builder
	if basePrompt != "" {
		sb.WriteString(strings.TrimSpace(basePrompt))
		sb.WriteString("\n\n")
	}

	sb.WriteString("=== Live Korean Standard Time (KST) & Calendar Context ===\n")
	sb.WriteString(fmt.Sprintf("- Current Time: %s (KST, UTC+9, Asia/Seoul)\n", now.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("- Today's Date: %s (%s)\n", hInfo.Date, hInfo.Weekday))
	sb.WriteString(fmt.Sprintf("- Calendar Status: %s\n", statusStr))
	sb.WriteString(fmt.Sprintf("- Is Business Day: %t | Is Public Holiday: %t | Is Weekend: %t\n\n", hInfo.IsBusinessDay, hInfo.IsHoliday, hInfo.IsWeekend))

	sb.WriteString("=== Time & Context Reasoning Instructions ===\n")
	sb.WriteString("1. Default Time Context: If the user's message does not explicitly provide a date or time, ALWAYS use the current KST time and date provided above as the default reference context.\n")
	sb.WriteString("2. Relative Time Calculations: When the user says '오늘' (today), '지금' (now), '내일' (tomorrow), '모레' (day after tomorrow), '이번 주' (this week), or mentions relative times (e.g. '퇴근 알림', '10분 뒤', '오후 6시'), calculate exact target times and dates based on the live KST context above.\n")
	sb.WriteString("3. Bus Schedules: If user asks about today's shuttle bus without specifying a time, determine the next available departure relative to current KST time.\n")
	sb.WriteString("4. Politeness & Formatting: Always respond naturally and politely in Korean, formatted neatly for mobile chat screens.\n")

	return sb.String()
}
