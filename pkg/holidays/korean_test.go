package holidays

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckDate(t *testing.T) {
	loc := GetKSTLocation()

	// 2026-09-01 (Tuesday) - Regular business day
	t1 := time.Date(2026, 9, 1, 10, 0, 0, 0, loc)
	info1 := CheckDate(t1)
	assert.Equal(t, "2026-09-01", info1.Date)
	assert.Equal(t, "화요일", info1.Weekday)
	assert.False(t, info1.IsHoliday)
	assert.False(t, info1.IsWeekend)
	assert.True(t, info1.IsBusinessDay)
	assert.Equal(t, "평일 (영업일)", info1.Description)

	// 2026-09-25 (Friday) - Chuseok holiday
	t2 := time.Date(2026, 9, 25, 12, 0, 0, 0, loc)
	info2 := CheckDate(t2)
	assert.Equal(t, "2026-09-25", info2.Date)
	assert.Equal(t, "금요일", info2.Weekday)
	assert.True(t, info2.IsHoliday)
	assert.Equal(t, "추석", info2.HolidayName)
	assert.False(t, info2.IsBusinessDay)

	// 2026-09-28 (Monday) - Chuseok substitute holiday
	t3 := time.Date(2026, 9, 28, 9, 0, 0, 0, loc)
	info3 := CheckDate(t3)
	assert.True(t, info3.IsHoliday)
	assert.True(t, info3.IsSubstituteHoliday)
	assert.Equal(t, "추석 대체공휴일", info3.HolidayName)
	assert.False(t, info3.IsBusinessDay)

	// 2026-08-15 (Saturday) - Gwangbokjeol
	t4 := time.Date(2026, 8, 15, 0, 0, 0, 0, loc)
	info4 := CheckDate(t4)
	assert.True(t, info4.IsHoliday)
	assert.True(t, info4.IsWeekend)
	assert.Equal(t, "광복절", info4.HolidayName)

	// 2026-08-17 (Monday) - Gwangbokjeol substitute holiday
	t5 := time.Date(2026, 8, 17, 0, 0, 0, 0, loc)
	info5 := CheckDate(t5)
	assert.True(t, info5.IsHoliday)
	assert.True(t, info5.IsSubstituteHoliday)
}

func TestParseAndCheck(t *testing.T) {
	info, err := ParseAndCheck("2026-10-09")
	require.NoError(t, err)
	assert.True(t, info.IsHoliday)
	assert.Equal(t, "한글날", info.HolidayName)

	infoToday, err := ParseAndCheck("오늘")
	require.NoError(t, err)
	assert.NotEmpty(t, infoToday.Date)

	_, err = ParseAndCheck("invalid-date-string-xyz")
	assert.Error(t, err)
}

func TestGetUpcomingHolidays(t *testing.T) {
	loc := GetKSTLocation()
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	upcoming := GetUpcomingHolidays(from, 5)
	require.Len(t, upcoming, 5)

	assert.Equal(t, "2026-09-24", upcoming[0].Date)
	assert.Equal(t, "추석 연휴", upcoming[0].HolidayName)
	assert.Equal(t, "2026-09-25", upcoming[1].Date)
	assert.Equal(t, "추석", upcoming[1].HolidayName)
	assert.Equal(t, "2026-09-26", upcoming[2].Date)
	assert.Equal(t, "추석 연휴", upcoming[2].HolidayName)
	assert.Equal(t, "2026-09-28", upcoming[3].Date)
	assert.Equal(t, "추석 대체공휴일", upcoming[3].HolidayName)
	assert.Equal(t, "2026-10-03", upcoming[4].Date)
	assert.Equal(t, "개천절", upcoming[4].HolidayName)
}
