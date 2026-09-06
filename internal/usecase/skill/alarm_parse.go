package skill

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/samber/lo"
)

var (
	clockTimeRegex  = regexp.MustCompile(`(\d{1,2})\s*[:：]\s*(\d{1,2})`)
	koreanHourRegex = regexp.MustCompile(`(\d{1,2})\s*시(?:\s*(\d{1,2})\s*분|\s*(반))?`)
	durationRegex   = regexp.MustCompile(`(\d+)\s*(시간|분|초)`)
)

// parseAlarmTime extracts a wall-clock time from Korean natural language such as
// "오후 3시 30분", "18시", or "15:00". Meridiem markers are applied explicitly, and
// unmarked hours are taken literally in 24-hour terms. ok is false when no valid
// time is present, so callers can refuse to schedule rather than guess.
func parseAlarmTime(text string) (hour, minute int, ok bool) {
	var h, m int
	switch {
	case clockTimeRegex.MatchString(text):
		match := clockTimeRegex.FindStringSubmatch(text)
		h, _ = strconv.Atoi(match[1])
		m, _ = strconv.Atoi(match[2])
	case koreanHourRegex.MatchString(text):
		loc := koreanHourRegex.FindStringSubmatchIndex(text)
		// "1시간" is a duration, not a wall-clock hour.
		if strings.HasPrefix(text[loc[1]:], "간") {
			return 0, 0, false
		}
		h, _ = strconv.Atoi(text[loc[2]:loc[3]])
		switch {
		case loc[4] != -1:
			m, _ = strconv.Atoi(text[loc[4]:loc[5]])
		case loc[6] != -1:
			m = 30
		}
	default:
		return 0, 0, false
	}

	switch {
	case strings.Contains(text, "오후") && h < 12:
		h += 12
	case strings.Contains(text, "오전") && h == 12:
		h = 0
	}

	if h > 23 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

// formatKoreanClock renders a 24-hour time as Korean meridiem notation (오후 3:05).
func formatKoreanClock(hour, minute int) string {
	meridiem, display := "오전", hour
	switch {
	case hour == 0:
		display = 12
	case hour == 12:
		meridiem = "오후"
	case hour > 12:
		meridiem, display = "오후", hour-12
	}
	return meridiem + " " + strconv.Itoa(display) + ":" + pad2(minute)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// parseAlarmRecurrence classifies the recurrence of an alarm request and returns
// the cron day-of-week field plus a human label matching the actual schedule.
func parseAlarmRecurrence(text string) (dow string, label string, ok bool) {
	switch {
	case strings.Contains(text, "평일") || strings.Contains(text, "월~금") || strings.Contains(text, "월요일부터"):
		return "1-5", "매주 평일", true
	case strings.Contains(text, "주말"):
		return "6,0", "주말", true
	case strings.Contains(text, "매일"):
		return "*", "매일", true
	}

	weekdays := []struct {
		word string
		dow  string
	}{
		{"월요일", "1"}, {"화요일", "2"}, {"수요일", "3"}, {"목요일", "4"},
		{"금요일", "5"}, {"토요일", "6"}, {"일요일", "0"},
	}
	matched, found := lo.Find(weekdays, func(d struct{ word, dow string }) bool {
		return strings.Contains(text, d.word)
	})
	if found {
		return matched.dow, "매주 " + matched.word, true
	}
	return "", "", false
}

var alarmNoiseWords = []string{
	"매주", "매일", "평일", "주말", "월~금", "오전", "오후",
	"월요일", "화요일", "수요일", "목요일", "금요일", "토요일", "일요일",
	"알림", "예약", "등록해줘", "등록해주세요", "설정해줘", "설정해주세요",
	"등록", "설정", "해줘", "해주세요", "주세요", "줘", "마다", "뒤에", "후에", "뒤", "후",
}

var alarmDropTokens = map[string]bool{
	"에": true, "은": true, "는": true, "가": true, "을": true,
	"를": true, "이": true, "내": true, "좀": true, "제발": true,
}

// alarmContentFromText strips scheduling tokens (times, recurrence words,
// politeness suffixes) from an alarm request and returns the user-facing subject.
func alarmContentFromText(text string) string {
	cleaned := clockTimeRegex.ReplaceAllString(text, " ")
	// Durations must be stripped before bare hours, or "1시간" loses only "1시"
	// and leaves a stray "간" behind.
	cleaned = durationRegex.ReplaceAllString(cleaned, " ")
	cleaned = koreanHourRegex.ReplaceAllString(cleaned, " ")
	for _, w := range alarmNoiseWords {
		cleaned = strings.ReplaceAll(cleaned, w, " ")
	}

	kept := lo.Filter(strings.Fields(cleaned), func(field string, _ int) bool {
		return !alarmDropTokens[field]
	})
	return strings.Join(kept, " ")
}

// truncateRunes shortens s to at most max runes, trading its last rune for an
// ellipsis when cut so the result never exceeds the limit.
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}
