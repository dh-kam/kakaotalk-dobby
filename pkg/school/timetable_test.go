package school

import (
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

