package skill

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/academy"
	"github.com/dh-kam/kakaotalk-dobby/pkg/agent"
	"github.com/dh-kam/kakaotalk-dobby/pkg/scheduler"
	"github.com/dh-kam/kakaotalk-dobby/pkg/school"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAdversarial_TenantIsolationAndPermissions verifies multi-user security boundary.
func TestAdversarial_TenantIsolationAndPermissions(t *testing.T) {
	schedEngine := scheduler.NewEngine(scheduler.NewMemoryStore(), func(ctx context.Context, job *scheduler.Job) error {
		return nil
	})
	require.NoError(t, schedEngine.Start(t.Context()))
	defer schedEngine.Stop()

	// Alice creates a scheduled job
	aliceJob, err := schedEngine.ScheduleOnce("alice", "Alice Private Job", "Alice Message", time.Now().Add(10*time.Minute), nil)
	require.NoError(t, err)

	reg := NewDefaultSlashCommandRegistry()

	// 1. Bob attempts to list Alice's jobs via /alarm list
	bobListResp := reg.Execute(&CommandContext{
		Command:     "alarm",
		Args:        []string{"list"},
		UserID:      "bob",
		SchedEngine: schedEngine,
	})
	require.NotNil(t, bobListResp)
	bobListText := extractResponseText(bobListResp)
	assert.NotContains(t, bobListText, aliceJob.ID)
	assert.Contains(t, bobListText, "등록된 예약 알림이 없습니다")

	// 2. Bob attempts to cancel Alice's specific job via /alarm cancel <alice_job_id>
	bobCancelResp := reg.Execute(&CommandContext{
		Command:     "alarm",
		Args:        []string{"cancel", aliceJob.ID},
		UserID:      "bob",
		SchedEngine: schedEngine,
	})
	require.NotNil(t, bobCancelResp)
	bobCancelText := extractResponseText(bobCancelResp)
	assert.Contains(t, bobCancelText, "취소 실패")

	// Verify Alice's job is still active
	activeJobs := schedEngine.ListJobs("alice")
	require.Len(t, activeJobs, 1)
	assert.Equal(t, scheduler.JobStatusActive, activeJobs[0].Status)

	// 3. Bob attempts to cancel all jobs via /alarm cancel all
	bobCancelAllResp := reg.Execute(&CommandContext{
		Command:     "alarm",
		Args:        []string{"cancel", "all"},
		UserID:      "bob",
		SchedEngine: schedEngine,
	})
	require.NotNil(t, bobCancelAllResp)
	bobCancelAllText := extractResponseText(bobCancelAllResp)
	assert.Contains(t, bobCancelAllText, "0건")

	// Verify Alice's job remains unharmed
	aliceJobsAfter := schedEngine.ListJobs("alice")
	require.Len(t, aliceJobsAfter, 1)
	assert.Equal(t, scheduler.JobStatusActive, aliceJobsAfter[0].Status)

	// 4. Anonymous / Empty UserID attempts to execute /alarm list or cancel
	emptyUserResp := reg.Execute(&CommandContext{
		Command:     "alarm",
		Args:        []string{"list"},
		UserID:      "   ",
		SchedEngine: schedEngine,
	})
	require.NotNil(t, emptyUserResp)
	assert.Contains(t, extractResponseText(emptyUserResp), "유효한 사용자")
}

// TestAdversarial_PayloadBombsAndMaliciousInputs tests resilience against extreme inputs.
func TestAdversarial_PayloadBombsAndMaliciousInputs(t *testing.T) {
	reg := NewDefaultSlashCommandRegistry()

	baseCtx := &CommandContext{
		UserID:    "adversary-01",
		ChannelID: "0xc0de1ab",
	}

	// 1. Oversized utterance (> 2,000 runes)
	giantBomb := "/" + strings.Repeat("A", 3000)
	bombResp := reg.Execute(&CommandContext{
		Command: "status",
		RawText: giantBomb,
		UserID:  "adversary-01",
	})
	require.NotNil(t, bombResp)
	assert.Contains(t, extractResponseText(bombResp), "길이가 제한")

	// 2. Injection attempts in commands and arguments
	injections := []struct {
		cmd  string
		args []string
	}{
		{cmd: "bus", args: []string{"' OR 1=1; DROP TABLE schedules; --"}},
		{cmd: "school", args: []string{"<script>alert('xss')</script>"}},
		{cmd: "alarm", args: []string{"cancel", "../../../../etc/passwd"}},
		{cmd: "data", args: []string{"reload; rm -rf /"}},
		{cmd: "data", args: []string{"../../../../etc"}},
		{cmd: "\x00\x00\x00", args: []string{"\x00"}},
		{cmd: "help\u202e", args: []string{"\u200b"}},
	}

	for _, inj := range injections {
		t.Run(fmt.Sprintf("cmd_%s", inj.cmd), func(t *testing.T) {
			ctx := *baseCtx
			ctx.Command = inj.cmd
			ctx.Args = inj.args
			resp := reg.Execute(&ctx)
			require.NotNil(t, resp)
			// Must return a valid OpenBuilder response without panicking
			assert.NotEmpty(t, resp.Template.Outputs)
			resp.ValidateAndNormalize()
		})
	}

	// 3. Massive argument count (1,000 arguments)
	manyArgs := make([]string, 1000)
	for i := range manyArgs {
		manyArgs[i] = fmt.Sprintf("arg%d", i)
	}
	ctxManyArgs := *baseCtx
	ctxManyArgs.Command = "bus"
	ctxManyArgs.Args = manyArgs
	respMany := reg.Execute(&ctxManyArgs)
	require.NotNil(t, respMany)
}

// TestAdversarial_NilDependencySafety ensures absolute zero-panic guarantee when services are nil.
func TestAdversarial_NilDependencySafety(t *testing.T) {
	reg := NewDefaultSlashCommandRegistry()

	// Empty context with all dependencies nil
	emptyCtx := &CommandContext{
		UserID:       "nil-tester",
		BusSvc:       nil,
		SchoolSvc:    nil,
		SchedEngine:  nil,
		SessionStore: nil,
		Registry:     nil,
		DataDir:      "",
	}

	commands := []string{"help", "reset", "status", "ping", "bus", "school", "alarm", "data", "holiday", "unknown"}
	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			ctx := *emptyCtx
			ctx.Command = cmd
			assert.NotPanics(t, func() {
				resp := reg.Execute(&ctx)
				require.NotNil(t, resp)
				resp.ValidateAndNormalize()
			})
		})
	}

	// Nil context itself
	assert.Nil(t, reg.Execute(nil))
}

// TestAdversarial_ConcurrentHammerAndAtomicReload runs high concurrency barrage.
func TestAdversarial_ConcurrentHammerAndAtomicReload(t *testing.T) {
	tempDir := t.TempDir()

	// Seed bus schedule
	busJSON := `{
		"academy": {
			"name": "산동정상",
			"vehicle_number": "2호차",
			"type": "등원"
		},
		"stops": [
			{
				"location": "우미린 2차",
				"schedules": { "초등부": "15:00" },
				"highlighted": true
			}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "bus.json"), []byte(busJSON), 0644))

	// Seed school timetable
	schoolJSON := `{
		"title": "구미원당초 시간표",
		"school_year": 2026,
		"grade": 4,
		"class_number": 2,
		"weekly_timetable": {
			"monday": {
				"day_name": "월요일",
				"schedule": [
					{ "period": 1, "subject": "국어", "time": "09:00~09:40" }
				]
			}
		}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "school.json"), []byte(schoolJSON), 0644))

	busSvc := academy.NewService()
	require.NoError(t, busSvc.LoadFromDir(tempDir))

	schoolSvc := school.NewService()
	require.NoError(t, schoolSvc.LoadFromDir(tempDir))

	schedEngine := scheduler.NewEngine(scheduler.NewMemoryStore(), func(ctx context.Context, job *scheduler.Job) error {
		return nil
	})
	require.NoError(t, schedEngine.Start(t.Context()))
	defer schedEngine.Stop()

	sessionStore := agent.NewMemorySessionStore(20)
	reg := NewDefaultSlashCommandRegistry()

	concurrency := 24
	iterations := 40
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for workerID := 0; workerID < concurrency; workerID++ {
		wID := workerID
		go func() {
			defer wg.Done()
			user := fmt.Sprintf("hammer-user-%d", wID)

			for i := 0; i < iterations; i++ {
				switch i % 8 {
				case 0:
					// Add alarm
					reg.Execute(&CommandContext{
						Command:     "alarm",
						Args:        []string{"add", "매주", "평일", "15:00", "정상어학원"},
						UserID:      user,
						SchedEngine: schedEngine,
					})
				case 1:
					// List alarm
					reg.Execute(&CommandContext{
						Command:     "alarm",
						Args:        []string{"list"},
						UserID:      user,
						SchedEngine: schedEngine,
					})
				case 2:
					// Cancel alarm
					reg.Execute(&CommandContext{
						Command:     "alarm",
						Args:        []string{"cancel", "all"},
						UserID:      user,
						SchedEngine: schedEngine,
					})
				case 3:
					// Query bus during concurrent reloads
					resp := reg.Execute(&CommandContext{
						Command: "bus",
						Args:    []string{"우미린"},
						UserID:  user,
						BusSvc:  busSvc,
					})
					// Bus response must NEVER be empty or panic even while reload is active
					require.NotNil(t, resp)
				case 4:
					// Query school timetable
					resp := reg.Execute(&CommandContext{
						Command:   "school",
						Args:      []string{"월요일"},
						UserID:    user,
						SchoolSvc: schoolSvc,
					})
					require.NotNil(t, resp)
				case 5:
					// Atomic reload in-flight
					resp := reg.Execute(&CommandContext{
						Command:   "data",
						Args:      []string{"reload"},
						UserID:    user,
						DataDir:   tempDir,
						BusSvc:    busSvc,
						SchoolSvc: schoolSvc,
					})
					require.NotNil(t, resp)
				case 6:
					// Status inspection
					reg.Execute(&CommandContext{
						Command:     "status",
						UserID:      user,
						BusSvc:      busSvc,
						SchoolSvc:   schoolSvc,
						SchedEngine: schedEngine,
					})
				case 7:
					// Session clear
					reg.Execute(&CommandContext{
						Command:      "reset",
						UserID:       user,
						SessionStore: sessionStore,
					})
				}
			}
		}()
	}

	wg.Wait()
}

// TestAdversarial_RapidConcurrentTurns_TryLockSLAPreservation verifies that when a user sends
// two requests in rapid succession, the second request does NOT block indefinitely and breach
// Kakao's 5s SLA, but instead returns a prompt graceful response within the SLA budget.
func TestAdversarial_RapidConcurrentTurns_TryLockSLAPreservation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	user := "rapid-user-123"

	// Simulate first request acquiring the lock
	unlock1, ok1 := globalUserLocks.tryLock(ctx, user, 200*time.Millisecond)
	require.True(t, ok1)
	defer unlock1()

	// Second request arrives and tries to acquire lock with 100ms wait budget
	start := time.Now()
	unlock2, ok2 := globalUserLocks.tryLock(ctx, user, 100*time.Millisecond)
	duration := time.Since(start)

	assert.False(t, ok2, "second concurrent request must fail to acquire lock within wait budget")
	assert.Nil(t, unlock2)
	assert.Less(t, duration, 300*time.Millisecond, "must abort wait quickly without hanging")

	// Anonymous requests should never be blocked by another anonymous request
	unlockAnon1, okAnon1 := globalUserLocks.tryLock(ctx, "anonymous", 50*time.Millisecond)
	require.True(t, okAnon1)
	defer unlockAnon1()

	unlockAnon2, okAnon2 := globalUserLocks.tryLock(ctx, "anonymous", 50*time.Millisecond)
	require.True(t, okAnon2)
	defer unlockAnon2()
}

