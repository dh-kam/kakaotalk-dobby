package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKoreanHolidayTool(t *testing.T) {
	tool := &KoreanHolidayTool{}
	assert.Equal(t, "check_korean_holiday", tool.Name())

	// Test Chuseok 2026
	output, err := tool.Execute(context.Background(), `{"date": "2026-09-25", "list_upcoming": true}`)
	require.NoError(t, err)

	var data map[string]interface{}
	err = json.Unmarshal([]byte(output), &data)
	require.NoError(t, err)

	assert.Equal(t, "2026-09-25", data["date"])
	assert.Equal(t, "금요일", data["weekday"])
	assert.Equal(t, true, data["is_holiday"])
	assert.Equal(t, "추석", data["holiday_name"])
	assert.Equal(t, false, data["is_business_day"])
	assert.NotEmpty(t, data["upcoming_holidays"])

	// Test today default
	outputToday, err := tool.Execute(context.Background(), `{}`)
	require.NoError(t, err)
	assert.NotEmpty(t, outputToday)
}
