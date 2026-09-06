package school

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/holidays"
	"github.com/samber/lo"
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
	mu         sync.RWMutex
	timetable  *Timetable
	timetables map[string]*Timetable
}

// NewService creates an empty school timetable Service.
func NewService() *Service {
	return &Service{
		timetables: make(map[string]*Timetable),
	}
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

	svc := NewService()
	svc.timetable = &tt
	svc.timetables[timetableKey(&tt)] = &tt
	return svc, nil
}

// timetableKey identifies a timetable by grade and class number.
func timetableKey(tt *Timetable) string {
	return fmt.Sprintf("%d-%d", tt.Grade, tt.ClassNumber)
}

// scanTimetablesDir reads every valid timetable JSON in dirPath keyed by grade-class.
func scanTimetablesDir(dirPath string) (map[string]*Timetable, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("read school timetables dir: %w", err)
	}

	timetables := make(map[string]*Timetable)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dirPath, entry.Name()))
		if err != nil {
			continue
		}

		var tt Timetable
		if err := json.Unmarshal(data, &tt); err != nil {
			continue
		}

		// Valid school timetable must have grade, class, and weekly timetable
		if tt.Grade > 0 && tt.ClassNumber > 0 && len(tt.WeeklyTimetable) > 0 {
			timetables[timetableKey(&tt)] = &tt
		}
	}
	return timetables, nil
}

// firstTimetable picks the lexicographically first entry for deterministic selection.
func firstTimetable(timetables map[string]*Timetable) *Timetable {
	keys := lo.Keys(timetables)
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	return timetables[keys[0]]
}

// LoadFromDir scans a directory and loads all school timetable JSON files.
func (s *Service) LoadFromDir(dirPath string) error {
	timetables, err := scanTimetablesDir(dirPath)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.timetables == nil {
		s.timetables = make(map[string]*Timetable)
	}
	maps.Copy(s.timetables, timetables)
	if s.timetable == nil {
		s.timetable = firstTimetable(s.timetables)
	}
	return nil
}

// ReloadFromDir reads timetable files from disk and atomically swaps in-memory state.
// The current primary is preserved when still present, otherwise selection is deterministic.
func (s *Service) ReloadFromDir(dirPath string) error {
	timetables, err := scanTimetablesDir(dirPath)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.timetable == nil {
		s.timetable = firstTimetable(timetables)
	} else if _, ok := timetables[timetableKey(s.timetable)]; !ok {
		s.timetable = firstTimetable(timetables)
	}
	s.timetables = timetables
	return nil
}

// GetSummary returns a human-readable list of registered school classes.
func (s *Service) GetSummary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.timetables) == 0 {
		if s.timetable != nil {
			return fmt.Sprintf("%d학년도 %d학년 %d반 (%s)", s.timetable.SchoolYear, s.timetable.Grade, s.timetable.ClassNumber, s.timetable.Title)
		}
		return ""
	}

	var items []string
	for _, tt := range s.timetables {
		items = append(items, fmt.Sprintf("%d학년도 %d학년 %d반", tt.SchoolYear, tt.Grade, tt.ClassNumber))
	}
	return strings.Join(items, ", ")
}

// GetTimetable returns the primary or default timetable model.
func (s *Service) GetTimetable() *Timetable {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.timetable
}

// GetScheduleForDay queries schedule for a given day query ("월", "화요일", "오늘", "내일", "wednesday", etc.)
func (s *Service) GetScheduleForDay(dayQuery string) (*DaySchedule, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.timetable == nil {
		return nil, "", fmt.Errorf("timetable data is not loaded")
	}

	now := time.Now().In(holidays.GetKSTLocation())
	key, err := dayQueryToKey(strings.TrimSpace(strings.ToLower(dayQuery)), now)
	if err != nil {
		return nil, "", err
	}
	if key == "weekend" {
		return nil, "", fmt.Errorf("주말(토/일)에는 학교 수업이 없습니다")
	}

	daySched, ok := s.timetable.WeeklyTimetable[key]
	if !ok {
		return nil, "", fmt.Errorf("해당 요일(%s)의 시간표 정보를 찾을 수 없습니다", dayQuery)
	}

	return &daySched, key, nil
}

type relativeDayWord struct {
	word   string
	offset int
}

type weekdayWord struct {
	word string
	key  string
}

// dayQueryToKey resolves a Korean or English day query to a weekly timetable
// key. Unrecognized queries return an error instead of silently defaulting to
// a weekday; single Korean characters match only on exact equality so free
// text such as "지금" never resolves to 금요일.
func dayQueryToKey(clean string, now time.Time) (string, error) {
	if clean == "" {
		return weekdayToKey(now.Weekday()), nil
	}

	weekdays := []weekdayWord{
		{"월요일", "monday"}, {"monday", "monday"},
		{"화요일", "tuesday"}, {"tuesday", "tuesday"},
		{"수요일", "wednesday"}, {"wednesday", "wednesday"},
		{"목요일", "thursday"}, {"thursday", "thursday"},
		{"금요일", "friday"}, {"friday", "friday"},
	}

	if matched, found := lo.Find([]relativeDayWord{
		{"오늘", 0}, {"today", 0},
		{"내일", 1}, {"tomorrow", 1},
		{"모레", 2},
		{"어제", -1},
	}, func(d relativeDayWord) bool {
		return strings.Contains(clean, d.word)
	}); found {
		return weekdayToKey(now.AddDate(0, 0, matched.offset).Weekday()), nil
	}

	if matched, found := lo.Find(weekdays, func(w weekdayWord) bool {
		return strings.Contains(clean, w.word)
	}); found {
		return matched.key, nil
	}

	switch clean {
	case "월":
		return "monday", nil
	case "화":
		return "tuesday", nil
	case "수":
		return "wednesday", nil
	case "목":
		return "thursday", nil
	case "금":
		return "friday", nil
	case "토", "일", "토요일", "일요일", "주말", "weekend":
		return "", fmt.Errorf("주말(토/일)에는 학교 수업이 없습니다")
	}

	if strings.Contains(clean, "토요일") || strings.Contains(clean, "일요일") || strings.Contains(clean, "주말") {
		return "", fmt.Errorf("주말(토/일)에는 학교 수업이 없습니다")
	}
	return "", fmt.Errorf("인식할 수 없는 요일입니다: %q (예: 월요일, 오늘, 내일)", clean)
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
