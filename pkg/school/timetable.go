package school

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/holidays"
)

// PeriodInfo describes time boundaries for a period.
type PeriodInfo struct {
	Period          *int   `json:"period"`
	Name            string `json:"name"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
	DurationMinutes int    `json:"duration_minutes"`
	Activity        string `json:"activity,omitempty"`
}

// LessonItem represents an individual class in a day.
type LessonItem struct {
	Period  *int   `json:"period"`
	Time    string `json:"time"`
	Subject string `json:"subject"`
}

// DaySchedule holds the schedule for one day of the week.
type DaySchedule struct {
	DayName         string       `json:"day_name"`
	DayShort        string       `json:"day_short"`
	TotalPeriods    int          `json:"total_periods"`
	DismissalTime   string       `json:"dismissal_time"`
	MorningActivity string       `json:"morning_activity"`
	Schedule        []LessonItem `json:"schedule"`
}

// Timetable represents the complete school timetable schema.
type Timetable struct {
	Title           string                 `json:"title"`
	SchoolYear      int                    `json:"school_year"`
	Grade           int                    `json:"grade"`
	ClassNumber     int                    `json:"class_number"`
	Description     string                 `json:"description"`
	Note            string                 `json:"note"`
	DailySchedule   DailyScheduleInfo      `json:"daily_schedule"`
	WeeklyTimetable map[string]DaySchedule `json:"weekly_timetable"`
	SubjectSummary  SubjectSummaryInfo     `json:"subject_summary"`
	ClassRules      ClassRulesInfo         `json:"class_rules"`
}

// DailyScheduleInfo defines standard bell schedule.
type DailyScheduleInfo struct {
	MorningActivity MorningActivityInfo `json:"morning_activity"`
	Periods         []PeriodInfo        `json:"periods"`
}

// MorningActivityInfo defines morning activity.
type MorningActivityInfo struct {
	Name      string `json:"name"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Activity  string `json:"activity"`
}

// SubjectSummaryInfo holds total weekly subject hours.
type SubjectSummaryInfo struct {
	TotalWeeklyHours int            `json:"total_weekly_hours"`
	HoursBySubject   map[string]int `json:"hours_by_subject"`
}

// ClassRulesInfo holds class rules.
type ClassRulesInfo struct {
	Title string   `json:"title"`
	Rules []string `json:"rules"`
}

// Service provides access to school timetable information.
type Service struct {
	timetable *Timetable
}

// NewServiceFromFile loads a school timetable from a JSON file.
func NewServiceFromFile(filePath string) (*Service, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read school timetable file %q: %w", filePath, err)
	}

	var tt Timetable
	if err := json.Unmarshal(data, &tt); err != nil {
		return nil, fmt.Errorf("unmarshal school timetable: %w", err)
	}

	return &Service{timetable: &tt}, nil
}

// GetTimetable returns the raw timetable model.
func (s *Service) GetTimetable() *Timetable {
	return s.timetable
}

// GetScheduleForDay queries schedule for a given day query ("월", "화요일", "오늘", "내일", "wednesday", etc.)
func (s *Service) GetScheduleForDay(dayQuery string) (*DaySchedule, string, error) {
	if s.timetable == nil {
		return nil, "", fmt.Errorf("timetable data is not loaded")
	}

	clean := strings.TrimSpace(strings.ToLower(dayQuery))
	now := time.Now().In(holidays.GetKSTLocation())

	var key string
	switch {
	case clean == "" || clean == "오늘" || clean == "today":
		key = weekdayToKey(now.Weekday())
	case clean == "내일" || clean == "tomorrow":
		key = weekdayToKey(now.AddDate(0, 0, 1).Weekday())
	case clean == "모레":
		key = weekdayToKey(now.AddDate(0, 0, 2).Weekday())
	case clean == "어제":
		key = weekdayToKey(now.AddDate(0, 0, -1).Weekday())
	case strings.Contains(clean, "월"):
		key = "monday"
	case strings.Contains(clean, "화"):
		key = "tuesday"
	case strings.Contains(clean, "수"):
		key = "wednesday"
	case strings.Contains(clean, "목"):
		key = "thursday"
	case strings.Contains(clean, "금"):
		key = "friday"
	case strings.Contains(clean, "토") || strings.Contains(clean, "일"):
		return nil, "", fmt.Errorf("주말(토/일)에는 학교 수업이 없습니다")
	default:
		key = "monday"
	}

	if key == "" || key == "weekend" {
		return nil, "", fmt.Errorf("주말(토/일)에는 학교 수업이 없습니다")
	}

	daySched, ok := s.timetable.WeeklyTimetable[key]
	if !ok {
		return nil, "", fmt.Errorf("해당 요일(%s)의 시간표 정보를 찾을 수 없습니다", dayQuery)
	}

	return &daySched, key, nil
}

func weekdayToKey(wd time.Weekday) string {
	switch wd {
	case time.Monday:
		return "monday"
	case time.Tuesday:
		return "tuesday"
	case time.Wednesday:
		return "wednesday"
	case time.Thursday:
		return "thursday"
	case time.Friday:
		return "friday"
	default:
		return "weekend"
	}
}
