package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dh-kam/kakaotalk-dobby/pkg/school"
)

// SchoolTimetableTool allows the Agent to lookup elementary school timetable (6학년 9반).
type SchoolTimetableTool struct {
	svc *school.Service
}

// NewSchoolTimetableTool creates a SchoolTimetableTool.
func NewSchoolTimetableTool(svc *school.Service) *SchoolTimetableTool {
	return &SchoolTimetableTool{svc: svc}
}

func (t *SchoolTimetableTool) Name() string {
	return "get_school_timetable"
}

func (t *SchoolTimetableTool) Description() string {
	return "Lookup the elementary school class timetable (6학년 9반), daily subjects, period times (1~6교시, 점심시간, 아침활동), dismissal/finish times, weekly subject hours, and classroom rules. Call this whenever the user asks about school schedule (e.g. '오늘 학교 시간표', '화요일 3교시 무슨 과목', '오늘 몇 교시에 끝나?', '수요일 하교 시간')."
}

func (t *SchoolTimetableTool) ParametersSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"day": map[string]interface{}{
				"type":        "string",
				"description": "Optional day query (e.g. '오늘', '내일', '월', '화요일', '수', '목요일', '금', '전체'). Defaults to '오늘' if not specified.",
			},
			"query_type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"schedule", "rules", "summary", "all"},
				"description": "Optional type of information requested: 'schedule' (default), 'rules' (학급 생활 규칙), 'summary' (주간 과목별 시수 통계), or 'all'.",
			},
		},
	}
}

func (t *SchoolTimetableTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	if t.svc == nil {
		return "", fmt.Errorf("school timetable service is not initialized")
	}

	var args struct {
		Day       string `json:"day"`
		QueryType string `json:"query_type"`
	}

	if argsJSON != "" && argsJSON != "{}" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid json arguments: %w", err)
		}
	}

	if args.Day == "" {
		args.Day = "오늘"
	}
	if args.QueryType == "" {
		args.QueryType = "schedule"
	}

	tt := t.svc.GetTimetable()
	if tt == nil {
		return "학교 시간표 데이터가 없습니다.", nil
	}

	if args.QueryType == "rules" {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📋 %s (6학년 %d반)\n", tt.ClassRules.Title, tt.ClassNumber))
		for i, r := range tt.ClassRules.Rules {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r))
		}
		return strings.TrimSpace(sb.String()), nil
	}

	if args.QueryType == "summary" {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📊 6학년 %d반 주간 과목별 시수 (총 %d시간):\n", tt.ClassNumber, tt.SubjectSummary.TotalWeeklyHours))
		for subj, hours := range tt.SubjectSummary.HoursBySubject {
			sb.WriteString(fmt.Sprintf("- %s: %d시간\n", subj, hours))
		}
		return strings.TrimSpace(sb.String()), nil
	}

	if args.Day == "전체" || args.Day == "all" || args.Day == "주간" {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("🏫 2026학년도 6학년 %d반 주간 시간표\n\n", tt.ClassNumber))
		days := []string{"monday", "tuesday", "wednesday", "thursday", "friday"}
		for _, d := range days {
			if ds, ok := tt.WeeklyTimetable[d]; ok {
				sb.WriteString(fmt.Sprintf("📌 [%s - 총 %d교시, 하교 %s]\n", ds.DayName, ds.TotalPeriods, ds.DismissalTime))
				for _, l := range ds.Schedule {
					if l.Period != nil {
						sb.WriteString(fmt.Sprintf("  %d교시 (%s): %s\n", *l.Period, l.Time, l.Subject))
					} else {
						sb.WriteString(fmt.Sprintf("  점심 (%s): %s\n", l.Time, l.Subject))
					}
				}
				sb.WriteString("\n")
			}
		}
		return strings.TrimSpace(sb.String()), nil
	}

	daySched, _, err := t.svc.GetScheduleForDay(args.Day)
	if err != nil {
		return err.Error(), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🏫 6학년 %d반 %s 시간표 (총 %d교시, 하교 %s)\n",
		tt.ClassNumber, daySched.DayName, daySched.TotalPeriods, daySched.DismissalTime))
	sb.WriteString(fmt.Sprintf("🌅 아침활동 (08:30-09:00): %s\n\n", daySched.MorningActivity))

	for _, l := range daySched.Schedule {
		if l.Period != nil {
			sb.WriteString(fmt.Sprintf("• %d교시 (%s): %s\n", *l.Period, l.Time, l.Subject))
		} else {
			sb.WriteString(fmt.Sprintf("🍱 점심시간 (%s): %s\n", l.Time, l.Subject))
		}
	}

	return strings.TrimSpace(sb.String()), nil
}
