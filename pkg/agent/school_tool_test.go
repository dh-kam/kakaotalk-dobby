package agent

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dh-kam/kakaotalk-dobby/pkg/school"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchoolTimetableTool(t *testing.T) {
	jsonPath := filepath.Join("..", "..", "data", "schedules", "2026_6-9_school_timetable.json")
	svc, err := school.NewServiceFromFile(jsonPath)
	require.NoError(t, err)

	tool := NewSchoolTimetableTool(svc)
	assert.Equal(t, "get_school_timetable", tool.Name())

	// Test query for Monday
	res, err := tool.Execute(context.Background(), `{"day":"월요일"}`)
	require.NoError(t, err)
	assert.Contains(t, res, "6학년 9반 월요일 시간표")
	assert.Contains(t, res, "국어")
	assert.Contains(t, res, "음악")

	// Test query for rules
	resRules, err := tool.Execute(context.Background(), `{"query_type":"rules"}`)
	require.NoError(t, err)
	assert.Contains(t, resRules, "우리반이 지켜야 할 하루 생활 규칙")
	assert.Contains(t, resRules, "독서")

	// Test query for summary
	resSum, err := tool.Execute(context.Background(), `{"query_type":"summary"}`)
	require.NoError(t, err)
	assert.Contains(t, resSum, "총 28시간")
	assert.Contains(t, resSum, "국어: 4시간")
}
