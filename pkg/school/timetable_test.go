package school

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchoolService_LoadAndQuery(t *testing.T) {
	jsonPath := filepath.Join("..", "..", "data", "schedules", "2026_6-9_school_timetable.json")
	svc, err := NewServiceFromFile(jsonPath)
	require.NoError(t, err)
	require.NotNil(t, svc)

	tt := svc.GetTimetable()
	assert.Equal(t, 2026, tt.SchoolYear)
	assert.Equal(t, 6, tt.Grade)
	assert.Equal(t, 9, tt.ClassNumber)
	assert.Equal(t, 28, tt.SubjectSummary.TotalWeeklyHours)

	// Query Monday
	mon, key, err := svc.GetScheduleForDay("월요일")
	require.NoError(t, err)
	assert.Equal(t, "monday", key)
	assert.Equal(t, "월요일", mon.DayName)
	assert.Equal(t, 6, mon.TotalPeriods)
	assert.Equal(t, "국어", mon.Schedule[0].Subject)
	assert.Equal(t, "영어", mon.Schedule[1].Subject)
	assert.Equal(t, "수학", mon.Schedule[2].Subject)

	// Query Wednesday (5 periods, early dismissal)
	wed, key, err := svc.GetScheduleForDay("수")
	require.NoError(t, err)
	assert.Equal(t, "wednesday", key)
	assert.Equal(t, 5, wed.TotalPeriods)
	assert.Equal(t, "13:20", wed.DismissalTime)

	// Query Weekend
	_, _, err = svc.GetScheduleForDay("토요일")
	assert.Error(t, err)
}

func TestSchoolService_LoadFromDirAndReload(t *testing.T) {
	dir := filepath.Join("..", "..", "data", "schedules")
	svc := NewService()
	err := svc.LoadFromDir(dir)
	require.NoError(t, err)

	summary := svc.GetSummary()
	assert.Contains(t, summary, "6학년 9반")

	tt := svc.GetTimetable()
	require.NotNil(t, tt)
	assert.Equal(t, 6, tt.Grade)

	// Test reload
	err = svc.ReloadFromDir(dir)
	require.NoError(t, err)
	assert.Contains(t, svc.GetSummary(), "6학년 9반")
}

func TestSchoolService_DayQueryRecognition(t *testing.T) {
	jsonPath := filepath.Join("..", "..", "data", "schedules", "2026_6-9_school_timetable.json")
	svc, err := NewServiceFromFile(jsonPath)
	require.NoError(t, err)

	// Free text that merely contains a weekday-looking character must not
	// resolve to that weekday.
	_, _, err = svc.GetScheduleForDay("지금 뭐해")
	assert.Error(t, err)
	_, _, err = svc.GetScheduleForDay("금방 갈게")
	assert.Error(t, err)

	// Exact single-character queries still resolve.
	_, key, err := svc.GetScheduleForDay("금")
	require.NoError(t, err)
	assert.Equal(t, "friday", key)

	// Weekend words are rejected explicitly, not silently defaulted.
	_, _, err = svc.GetScheduleForDay("일요일")
	assert.ErrorContains(t, err, "주말")

	// English full weekday names resolve.
	_, key, err = svc.GetScheduleForDay("wednesday")
	require.NoError(t, err)
	assert.Equal(t, "wednesday", key)
}

func TestSchoolService_ReloadPreservesPrimaryTimetable(t *testing.T) {
	dir := t.TempDir()
	writeTimetable := func(file string, grade, class int) {
		tt := map[string]any{
			"title": "테스트 시간표", "school_year": 2026,
			"grade": grade, "class_number": class,
			"weekly_timetable": map[string]any{
				"monday": map[string]any{"day_name": "월요일", "schedule": []any{}},
			},
		}
		data, err := json.Marshal(tt)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, file), data, 0o644))
	}

	writeTimetable("a.json", 6, 9)
	svc := NewService()
	require.NoError(t, svc.LoadFromDir(dir))
	require.NotNil(t, svc.GetTimetable())
	assert.Equal(t, 9, svc.GetTimetable().ClassNumber)

	// Removing the primary file from disk falls back deterministically
	// rather than leaving a dangling pointer.
	require.NoError(t, os.Remove(filepath.Join(dir, "a.json")))
	writeTimetable("b.json", 5, 2)
	require.NoError(t, svc.ReloadFromDir(dir))
	require.NotNil(t, svc.GetTimetable())
	assert.Equal(t, 2, svc.GetTimetable().ClassNumber)
}
