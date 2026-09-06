package skill

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAlarmTime(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		hour   int
		minute int
		ok     bool
	}{
		{name: "24h clock", text: "15:00 알림", hour: 15, minute: 0, ok: true},
		{name: "clock fullwidth colon", text: "09：30 알림", hour: 9, minute: 30, ok: true},
		{name: "korean hour with minutes", text: "오후 3시 30분 알림", hour: 15, minute: 30, ok: true},
		{name: "korean hour half", text: "오후 7시 반 알림", hour: 19, minute: 30, ok: true},
		{name: "meridiem noon", text: "오전 12시 알림", hour: 0, minute: 0, ok: true},
		{name: "afternoon noon", text: "오후 12시 알림", hour: 12, minute: 0, ok: true},
		{name: "literal 24h korean", text: "18시 알림", hour: 18, minute: 0, ok: true},
		{name: "no meridiem stays literal", text: "8시 알림", hour: 8, minute: 0, ok: true},
		{name: "invalid hour", text: "25시 알림", ok: false},
		{name: "invalid minute", text: "12:70 알림", ok: false},
		{name: "no time at all", text: "알림 등록해줘", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hour, minute, ok := parseAlarmTime(tt.text)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.hour, hour)
				assert.Equal(t, tt.minute, minute)
			}
		})
	}
}

func TestParseAlarmRecurrence(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		dow   string
		label string
		ok    bool
	}{
		{name: "weekdays", text: "매주 평일 15:00 알림", dow: "1-5", label: "매주 평일", ok: true},
		{name: "weekday range", text: "월~금 알림", dow: "1-5", label: "매주 평일", ok: true},
		{name: "weekend", text: "주말 알림", dow: "6,0", label: "주말", ok: true},
		{name: "daily", text: "매일 알림", dow: "*", label: "매일", ok: true},
		{name: "specific weekday", text: "매주 금요일 알림", dow: "5", label: "매주 금요일", ok: true},
		{name: "none", text: "10분 뒤 알림", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dow, label, ok := parseAlarmRecurrence(tt.text)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.dow, dow)
				assert.Equal(t, tt.label, label)
			}
		})
	}
}

func TestAlarmContentFromText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "recurring bus alarm keeps subject",
			text: "매주 평일 오후 3시에 정상어학원 2호차 등원 알림 등록해줘",
			want: "정상어학원 2호차 등원",
		},
		{
			name: "relative alarm keeps subject",
			text: "10분 뒤에 라면 물 끄기 알림 줘",
			want: "라면 물 끄기",
		},
		{
			name: "no-space relative duration",
			text: "1시간30분뒤 세탁기 확인 알림",
			want: "세탁기 확인",
		},
		{
			name: "duration without marker is still stripped",
			text: "매일 1시간 알림",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, alarmContentFromText(tt.text))
		})
	}
}

func TestFormatKoreanClock(t *testing.T) {
	assert.Equal(t, "오전 12:05", formatKoreanClock(0, 5))
	assert.Equal(t, "오전 9:00", formatKoreanClock(9, 0))
	assert.Equal(t, "오후 12:30", formatKoreanClock(12, 30))
	assert.Equal(t, "오후 3:05", formatKoreanClock(15, 5))
}

func TestTruncateRunes(t *testing.T) {
	assert.Equal(t, "가나다", truncateRunes("가나다", 10))
	assert.Equal(t, "가나…", truncateRunes("가나다라", 3))
}
