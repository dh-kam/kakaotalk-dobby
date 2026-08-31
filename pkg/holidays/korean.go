package holidays

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// HolidayInfo represents details about a specific date's holiday status in South Korea.
type HolidayInfo struct {
	Date                string `json:"date"`
	Year                int    `json:"year"`
	Month               int    `json:"month"`
	Day                 int    `json:"day"`
	Weekday             string `json:"weekday"`
	IsHoliday           bool   `json:"is_holiday"`
	IsWeekend           bool   `json:"is_weekend"`
	IsBusinessDay       bool   `json:"is_business_day"`
	HolidayName         string `json:"holiday_name,omitempty"`
	IsSubstituteHoliday bool   `json:"is_substitute_holiday,omitempty"`
	Description         string `json:"description"`
}

// knownHolidays stores confirmed Korean public holidays and substitute holidays (2024~2030).
var knownHolidays = map[string]struct {
	Name         string
	IsSubstitute bool
}{
	// 2024
	"2024-01-01": {Name: "신정 (새해 첫날)"},
	"2024-02-09": {Name: "설날 연휴"},
	"2024-02-10": {Name: "설날"},
	"2024-02-11": {Name: "설날 연휴"},
	"2024-02-12": {Name: "설날 대체공휴일", IsSubstitute: true},
	"2024-03-01": {Name: "3·1절"},
	"2024-04-10": {Name: "제22대 국회의원선거일"},
	"2024-05-05": {Name: "어린이날"},
	"2024-05-06": {Name: "어린이날 대체공휴일", IsSubstitute: true},
	"2024-05-15": {Name: "부처님오신날"},
	"2024-06-06": {Name: "현충일"},
	"2024-08-15": {Name: "광복절"},
	"2024-09-16": {Name: "추석 연휴"},
	"2024-09-17": {Name: "추석"},
	"2024-09-18": {Name: "추석 연휴"},
	"2024-10-01": {Name: "국군의 날 (임시공휴일)"},
	"2024-10-03": {Name: "개천절"},
	"2024-10-09": {Name: "한글날"},
	"2024-12-25": {Name: "성탄절 (크리스마스)"},

	// 2025
	"2025-01-01": {Name: "신정 (새해 첫날)"},
	"2025-01-28": {Name: "설날 연휴"},
	"2025-01-29": {Name: "설날"},
	"2025-01-30": {Name: "설날 연휴"},
	"2025-03-01": {Name: "3·1절"},
	"2025-03-03": {Name: "3·1절 대체공휴일", IsSubstitute: true},
	"2025-05-05": {Name: "어린이날 & 부처님오신날"},
	"2025-05-06": {Name: "대체공휴일 (어린이날/부처님오신날)", IsSubstitute: true},
	"2025-06-06": {Name: "현충일"},
	"2025-08-15": {Name: "광복절"},
	"2025-10-03": {Name: "개천절"},
	"2025-10-05": {Name: "추석 연휴"},
	"2025-10-06": {Name: "추석"},
	"2025-10-07": {Name: "추석 연휴"},
	"2025-10-08": {Name: "추석 대체공휴일", IsSubstitute: true},
	"2025-10-09": {Name: "한글날"},
	"2025-12-25": {Name: "성탄절 (크리스마스)"},

	// 2026
	"2026-01-01": {Name: "신정 (새해 첫날)"},
	"2026-02-16": {Name: "설날 연휴"},
	"2026-02-17": {Name: "설날"},
	"2026-02-18": {Name: "설날 연휴"},
	"2026-03-01": {Name: "3·1절"},
	"2026-03-02": {Name: "3·1절 대체공휴일", IsSubstitute: true},
	"2026-05-05": {Name: "어린이날"},
	"2026-05-24": {Name: "부처님오신날"},
	"2026-05-25": {Name: "부처님오신날 대체공휴일", IsSubstitute: true},
	"2026-06-03": {Name: "제9회 전국동시지방선거일"},
	"2026-06-06": {Name: "현충일"},
	"2026-08-15": {Name: "광복절"},
	"2026-08-17": {Name: "광복절 대체공휴일", IsSubstitute: true},
	"2026-09-24": {Name: "추석 연휴"},
	"2026-09-25": {Name: "추석"},
	"2026-09-26": {Name: "추석 연휴"},
	"2026-09-28": {Name: "추석 대체공휴일", IsSubstitute: true},
	"2026-10-03": {Name: "개천절"},
	"2026-10-05": {Name: "개천절 대체공휴일", IsSubstitute: true},
	"2026-10-09": {Name: "한글날"},
	"2026-12-25": {Name: "성탄절 (크리스마스)"},

	// 2027
	"2027-01-01": {Name: "신정 (새해 첫날)"},
	"2027-02-06": {Name: "설날 연휴"},
	"2027-02-07": {Name: "설날"},
	"2027-02-08": {Name: "설날 연휴"},
	"2027-02-09": {Name: "설날 대체공휴일", IsSubstitute: true},
	"2027-03-01": {Name: "3·1절"},
	"2027-03-03": {Name: "제21대 대통령선거일"},
	"2027-05-05": {Name: "어린이날"},
	"2027-05-13": {Name: "부처님오신날"},
	"2027-06-06": {Name: "현충일"},
	"2027-08-15": {Name: "광복절"},
	"2027-08-16": {Name: "광복절 대체공휴일", IsSubstitute: true},
	"2027-09-14": {Name: "추석 연휴"},
	"2027-09-15": {Name: "추석"},
	"2027-09-16": {Name: "추석 연휴"},
	"2027-10-03": {Name: "개천절"},
	"2027-10-04": {Name: "개천절 대체공휴일", IsSubstitute: true},
	"2027-10-09": {Name: "한글날"},
	"2027-10-11": {Name: "한글날 대체공휴일", IsSubstitute: true},
	"2027-12-25": {Name: "성탄절 (크리스마스)"},
	"2027-12-27": {Name: "성탄절 대체공휴일", IsSubstitute: true},

	// 2028
	"2028-01-01": {Name: "신정 (새해 첫날)"},
	"2028-01-26": {Name: "설날 연휴"},
	"2028-01-27": {Name: "설날"},
	"2028-01-28": {Name: "설날 연휴"},
	"2028-03-01": {Name: "3·1절"},
	"2028-04-12": {Name: "제23대 국회의원선거일"},
	"2028-05-02": {Name: "부처님오신날"},
	"2028-05-05": {Name: "어린이날"},
	"2028-06-06": {Name: "현충일"},
	"2028-08-15": {Name: "광복절"},
	"2028-10-02": {Name: "추석 연휴"},
	"2028-10-03": {Name: "개천절 & 추석"},
	"2028-10-04": {Name: "추석 연휴"},
	"2028-10-05": {Name: "추석 대체공휴일", IsSubstitute: true},
	"2028-10-09": {Name: "한글날"},
	"2028-12-25": {Name: "성탄절 (크리스마스)"},

	// 2029
	"2029-01-01": {Name: "신정 (새해 첫날)"},
	"2029-02-12": {Name: "설날 연휴"},
	"2029-02-13": {Name: "설날"},
	"2029-02-14": {Name: "설날 연휴"},
	"2029-03-01": {Name: "3·1절"},
	"2029-05-05": {Name: "어린이날"},
	"2029-05-07": {Name: "어린이날 대체공휴일", IsSubstitute: true},
	"2029-05-20": {Name: "부처님오신날"},
	"2029-05-21": {Name: "부처님오신날 대체공휴일", IsSubstitute: true},
	"2029-06-06": {Name: "현충일"},
	"2029-08-15": {Name: "광복절"},
	"2029-09-21": {Name: "추석 연휴"},
	"2029-09-22": {Name: "추석"},
	"2029-09-23": {Name: "추석 연휴"},
	"2029-09-24": {Name: "추석 대체공휴일", IsSubstitute: true},
	"2029-10-03": {Name: "개천절"},
	"2029-10-09": {Name: "한글날"},
	"2029-12-25": {Name: "성탄절 (크리스마스)"},

	// 2030
	"2030-01-01": {Name: "신정 (새해 첫날)"},
	"2030-02-02": {Name: "설날 연휴"},
	"2030-02-03": {Name: "설날"},
	"2030-02-04": {Name: "설날 연휴"},
	"2030-02-05": {Name: "설날 대체공휴일", IsSubstitute: true},
	"2030-03-01": {Name: "3·1절"},
	"2030-05-05": {Name: "어린이날"},
	"2030-05-06": {Name: "어린이날 대체공휴일", IsSubstitute: true},
	"2030-05-09": {Name: "부처님오신날"},
	"2030-06-06": {Name: "현충일"},
	"2030-08-15": {Name: "광복절"},
	"2030-09-11": {Name: "추석 연휴"},
	"2030-09-12": {Name: "추석"},
	"2030-09-13": {Name: "추석 연휴"},
	"2030-10-03": {Name: "개천절"},
	"2030-10-09": {Name: "한글날"},
	"2030-12-25": {Name: "성탄절 (크리스마스)"},
}

// Fixed solar holidays fallback for years beyond 2030
var solarHolidays = map[string]string{
	"01-01": "신정 (새해 첫날)",
	"03-01": "3·1절",
	"05-05": "어린이날",
	"06-06": "현충일",
	"08-15": "광복절",
	"10-03": "개천절",
	"10-09": "한글날",
	"12-25": "성탄절 (크리스마스)",
}

var koreanWeekdays = []string{"일요일", "월요일", "화요일", "수요일", "목요일", "금요일", "토요일"}

// GetKSTLocation returns Asia/Seoul timezone location safely.
func GetKSTLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.FixedZone("KST", 9*60*60)
	}
	return loc
}

// CheckDate evaluates if the given date is a Korean public holiday, weekend, or business day.
func CheckDate(t time.Time) HolidayInfo {
	kst := t.In(GetKSTLocation())
	dateStr := kst.Format("2006-01-02")
	monthDay := kst.Format("01-02")
	weekday := koreanWeekdays[kst.Weekday()]
	isWeekend := kst.Weekday() == time.Saturday || kst.Weekday() == time.Sunday

	var isHoliday bool
	var holidayName string
	var isSubstitute bool

	// 1. Check known table
	if h, ok := knownHolidays[dateStr]; ok {
		isHoliday = true
		holidayName = h.Name
		isSubstitute = h.IsSubstitute
	} else if name, ok := solarHolidays[monthDay]; ok {
		// Fallback to fixed solar holidays
		isHoliday = true
		holidayName = name
	}

	isBusinessDay := !isWeekend && !isHoliday

	var desc string
	if isHoliday {
		desc = fmt.Sprintf("공휴일 (%s)", holidayName)
	} else if isWeekend {
		desc = fmt.Sprintf("주말 (%s)", weekday)
	} else {
		desc = "평일 (영업일)"
	}

	return HolidayInfo{
		Date:                dateStr,
		Year:                kst.Year(),
		Month:               int(kst.Month()),
		Day:                 kst.Day(),
		Weekday:             weekday,
		IsHoliday:           isHoliday,
		IsWeekend:           isWeekend,
		IsBusinessDay:       isBusinessDay,
		HolidayName:         holidayName,
		IsSubstituteHoliday: isSubstitute,
		Description:         desc,
	}
}

// ParseAndCheck parses user input date (e.g. "2026-09-01", "20260901", "오늘", "내일") in KST and checks holiday status.
func ParseAndCheck(input string) (HolidayInfo, error) {
	now := time.Now().In(GetKSTLocation())
	clean := strings.TrimSpace(input)

	if clean == "" || clean == "오늘" || clean == "today" || clean == "현재" || strings.Contains(clean, "오늘") {
		return CheckDate(now), nil
	}
	if clean == "내일" || clean == "tomorrow" || strings.Contains(clean, "내일") {
		return CheckDate(now.AddDate(0, 0, 1)), nil
	}
	if clean == "모레" || strings.Contains(clean, "모레") {
		return CheckDate(now.AddDate(0, 0, 2)), nil
	}
	if clean == "글피" || strings.Contains(clean, "글피") {
		return CheckDate(now.AddDate(0, 0, 3)), nil
	}
	if clean == "어제" || clean == "yesterday" || strings.Contains(clean, "어제") {
		return CheckDate(now.AddDate(0, 0, -1)), nil
	}
	if strings.Contains(clean, "이번주 토요일") || strings.Contains(clean, "이번 주 토요일") {
		offset := (int(time.Saturday) - int(now.Weekday()) + 7) % 7
		return CheckDate(now.AddDate(0, 0, offset)), nil
	}
	if strings.Contains(clean, "이번주 일요일") || strings.Contains(clean, "이번 주 일요일") || strings.Contains(clean, "이번 주말") || strings.Contains(clean, "이번주말") {
		offset := (int(time.Sunday) - int(now.Weekday()) + 7) % 7
		return CheckDate(now.AddDate(0, 0, offset)), nil
	}

	// Natural Korean regex: e.g. "2026년 9월 25일", "9월 25일", "10월 9일"
	reYMD := regexp.MustCompile(`(\d{4})년\s*(\d{1,2})월\s*(\d{1,2})일`)
	if m := reYMD.FindStringSubmatch(clean); len(m) == 4 {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		t := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, GetKSTLocation())
		return CheckDate(t), nil
	}

	reMD := regexp.MustCompile(`(\d{1,2})월\s*(\d{1,2})일`)
	if m := reMD.FindStringSubmatch(clean); len(m) == 3 {
		mo, _ := strconv.Atoi(m[1])
		d, _ := strconv.Atoi(m[2])
		t := time.Date(now.Year(), time.Month(mo), d, 0, 0, 0, 0, GetKSTLocation())
		return CheckDate(t), nil
	}

	// Try standard date formats
	formats := []string{
		"2006-01-02",
		"2006.01.02",
		"2006/01/02",
		"20060102",
		"01-02",
		"01.02",
		"01/02",
		"0102",
	}

	for _, f := range formats {
		if t, err := time.ParseInLocation(f, clean, GetKSTLocation()); err == nil {
			if len(clean) <= 5 { // mm-dd format -> assume current year
				t = time.Date(now.Year(), t.Month(), t.Day(), 0, 0, 0, 0, GetKSTLocation())
			}
			return CheckDate(t), nil
		}
	}

	return HolidayInfo{}, fmt.Errorf("unable to parse date %q (expected YYYY-MM-DD, 9월 25일, 오늘, 내일, etc.)", input)
}

// GetUpcomingHolidays returns a list of upcoming Korean holidays starting from a reference date.
func GetUpcomingHolidays(from time.Time, count int) []HolidayInfo {
	if count <= 0 {
		count = 5
	}
	kst := from.In(GetKSTLocation())
	var results []HolidayInfo

	for i := 0; i < 365 && len(results) < count; i++ {
		target := kst.AddDate(0, 0, i)
		info := CheckDate(target)
		if info.IsHoliday {
			results = append(results, info)
		}
	}
	return results
}
