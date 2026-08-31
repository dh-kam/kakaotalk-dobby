package agent

import (
	"context"
	"testing"
	"time"

	"github.com/dh-kam/kakao-bot/pkg/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduleNotificationTool_OnceAndList(t *testing.T) {
	store := scheduler.NewMemoryStore()
	engine := scheduler.NewEngine(store, func(ctx context.Context, job *scheduler.Job) error {
		return nil
	})
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop()

	schedTool := NewScheduleNotificationTool(engine)
	listTool := NewListSchedulesTool(engine)
	cancelTool := NewCancelScheduleTool(engine)

	// 1. Schedule one-shot
	res, err := schedTool.Execute(context.Background(), `{"title":"정상어학원 등원 알림","message":"우미린 2차 버스 탑승 3분 전","time_type":"once","execute_at":"+1h"}`)
	require.NoError(t, err)
	assert.Contains(t, res, "알림이 성공적으로 예약되었습니다")

	// 2. List schedules
	listRes, err := listTool.Execute(context.Background(), `{}`)
	require.NoError(t, err)
	assert.Contains(t, listRes, "정상어학원 등원 알림")

	// 3. Cancel
	jobs := engine.ListJobs("")
	require.Len(t, jobs, 1)
	jobID := jobs[0].ID

	cancelRes, err := cancelTool.Execute(context.Background(), `{"job_id":"`+jobID+`"}`)
	require.NoError(t, err)
	assert.Contains(t, cancelRes, "성공적으로 취소되었습니다")
}

func TestParseExecuteTime(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Seoul")
	now := time.Now().In(loc)

	// +10m
	t1, err := parseExecuteTime("+10m", loc)
	require.NoError(t, err)
	assert.WithinDuration(t, now.Add(10*time.Minute), t1, 2*time.Second)

	// 15분 뒤
	t2, err := parseExecuteTime("15분 뒤", loc)
	require.NoError(t, err)
	assert.WithinDuration(t, now.Add(15*time.Minute), t2, 2*time.Second)

	// 15:30 (time of day)
	t3, err := parseExecuteTime("15:30", loc)
	require.NoError(t, err)
	assert.Equal(t, 15, t3.Hour())
	assert.Equal(t, 30, t3.Minute())
}
