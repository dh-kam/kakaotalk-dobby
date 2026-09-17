package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dh-kam/kakaotalk-dobby/pkg/academy"
	"github.com/dh-kam/kakaotalk-dobby/pkg/agent"
	"github.com/dh-kam/kakaotalk-dobby/pkg/ai"
	"github.com/dh-kam/kakaotalk-dobby/pkg/openbuilder"
	"github.com/dh-kam/kakaotalk-dobby/pkg/scheduler"
	"github.com/dh-kam/kakaotalk-dobby/pkg/school"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlashCommand_Parsing(t *testing.T) {
	tests := []struct {
		name         string
		utterance    string
		expectedCmd  string
		expectedArgs []string
	}{
		{
			name:         "simple command",
			utterance:    "/help",
			expectedCmd:  "help",
			expectedArgs: []string{},
		},
		{
			name:         "command with multiple arguments",
			utterance:    "/bus 정상어학원 우미린 2차",
			expectedCmd:  "bus",
			expectedArgs: []string{"정상어학원", "우미린", "2차"},
		},
		{
			name:         "command with extra slashes and whitespace",
			utterance:    "  ///status  detail   --all  ",
			expectedCmd:  "status",
			expectedArgs: []string{"detail", "--all"},
		},
		{
			name:         "uppercase command normalized",
			utterance:    "/RESET",
			expectedCmd:  "reset",
			expectedArgs: []string{},
		},
		{
			name:         "bare slash defaults to help",
			utterance:    "/",
			expectedCmd:  "help",
			expectedArgs: []string{},
		},
		{
			name:         "non-slash utterance returns empty",
			utterance:    "오늘 학교 시간표",
			expectedCmd:  "",
			expectedArgs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, args := parseSlashCommand(tt.utterance)
			assert.Equal(t, tt.expectedCmd, cmd)
			if tt.expectedArgs == nil {
				assert.Nil(t, args)
			} else {
				assert.Equal(t, tt.expectedArgs, args)
			}
		})
	}
}

func TestSlashCommand_Registry(t *testing.T) {
	reg := NewSlashCommandRegistry()

	customSpec := CommandSpec{
		Name:        "echo",
		Aliases:     []string{"say", "print"},
		Description: "Echo input back",
		Usage:       "/echo <text>",
		Category:    "Test",
		Handler: func(cc *CommandContext) *openbuilder.SkillResponse {
			text := "echo"
			if len(cc.Args) > 0 {
				text = cc.Args[0]
			}
			return openbuilder.NewSimpleTextResponse(text)
		},
	}
	reg.Register(customSpec)

	// Direct lookup
	spec, ok := reg.Find("echo")
	require.True(t, ok)
	assert.Equal(t, "echo", spec.Name)

	// Alias lookup
	specByAlias, ok := reg.Find("say")
	require.True(t, ok)
	assert.Equal(t, "echo", specByAlias.Name)

	specByAlias2, ok := reg.Find("PRINT")
	require.True(t, ok)
	assert.Equal(t, "echo", specByAlias2.Name)

	// List check
	list := reg.List()
	assert.Len(t, list, 1)

	// Execution
	resp := reg.Execute(&CommandContext{
		Command: "say",
		Args:    []string{"hello-world"},
	})
	require.NotNil(t, resp)
	assert.Equal(t, "hello-world", resp.Template.Outputs[0].SimpleText.Text)

	// Unknown command fallback
	unknownResp := reg.Execute(&CommandContext{
		Command: "nonexistent",
	})
	require.NotNil(t, unknownResp)
	assert.Contains(t, unknownResp.Template.Outputs[0].SimpleText.Text, "알 수 없는 명령어")
	assert.GreaterOrEqual(t, len(unknownResp.Template.QuickReplies), 1)
}

func TestSlashCommand_DefaultCommands(t *testing.T) {
	tempDir := t.TempDir()

	// Seed bus schedule
	busJSON := `{
		"academy": {
			"name": "정상어학원",
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
		"title": "2026학년도 4학년 2반 시간표",
		"school_year": 2026,
		"grade": 4,
		"class_number": 2,
		"weekly_timetable": {
			"monday": {
				"day_name": "월요일",
				"schedule": [
					{ "period": 1, "subject": "도덕", "time": "09:00~09:40" },
					{ "period": 2, "subject": "사회", "time": "09:50~10:30" }
				],
				"dismissal_time": "14:10"
			}
		},
		"class_rules": {
			"title": "학급 생활 규칙",
			"rules": ["바른 말 고운 말 사용하기", "안전하게 복도 걷기"]
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

	sessionStore := agent.NewMemorySessionStore(10)
	sessionStore.AppendTurn("user-1", "이전 질문", "이전 답변")
	assert.NotEmpty(t, sessionStore.GetHistory("user-1"))

	reg := NewDefaultSlashCommandRegistry()

	baseCtx := func(cmd string, args ...string) *CommandContext {
		return &CommandContext{
			Ctx:          t.Context(),
			Command:      cmd,
			Args:         args,
			UserID:       "user-1",
			ChannelID:    "0xc0de1ab",
			DataDir:      tempDir,
			BusSvc:       busSvc,
			SchoolSvc:    schoolSvc,
			SchedEngine:  schedEngine,
			SessionStore: sessionStore,
		}
	}

	// 1. /help
	helpResp := reg.Execute(baseCtx("help"))
	require.NotNil(t, helpResp)
	helpText := helpResp.Template.Outputs[0].SimpleText.Text
	assert.Contains(t, helpText, "KakaoBot Dobby 슬래시 명령어 안내")
	assert.Contains(t, helpText, "/help")
	assert.Contains(t, helpText, "/bus")
	assert.Contains(t, helpText, "/school")
	assert.Contains(t, helpText, "/alarm")
	assert.Contains(t, helpText, "/status")
	assert.NotEmpty(t, helpResp.Template.QuickReplies)

	// 2. /reset
	resetResp := reg.Execute(baseCtx("reset"))
	require.NotNil(t, resetResp)
	assert.Contains(t, resetResp.Template.Outputs[0].SimpleText.Text, "초기화했습니다")
	assert.Empty(t, sessionStore.GetHistory("user-1"))

	// 3. /status
	statusResp := reg.Execute(baseCtx("status"))
	require.NotNil(t, statusResp)
	assert.NotNil(t, statusResp.Template.Outputs[0].BasicCard)
	assert.Contains(t, statusResp.Template.Outputs[0].BasicCard.Description, "Goroutines")
	assert.Contains(t, statusResp.Template.Outputs[0].BasicCard.Description, "학원 노선: 1개")

	// 4. /ping
	pingResp := reg.Execute(baseCtx("ping"))
	require.NotNil(t, pingResp)
	assert.Contains(t, pingResp.Template.Outputs[0].SimpleText.Text, "퐁! (Pong)")

	// 5. /bus (no args)
	busNoArgsResp := reg.Execute(baseCtx("bus"))
	require.NotNil(t, busNoArgsResp)
	assert.Contains(t, busNoArgsResp.Template.Outputs[0].SimpleText.Text, "등록된 학원 버스 노선 목록")
	assert.Contains(t, busNoArgsResp.Template.Outputs[0].SimpleText.Text, "정상어학원")

	// 6. /bus with stop query
	busQueryResp := reg.Execute(baseCtx("bus", "우미린", "2차"))
	require.NotNil(t, busQueryResp)
	busQueryText := extractResponseText(busQueryResp)
	assert.Contains(t, busQueryText, "우미린 2차")
	assert.Contains(t, busQueryText, "15:00")

	// 7. /school rules query
	schoolRulesResp := reg.Execute(baseCtx("school", "규칙"))
	require.NotNil(t, schoolRulesResp)
	rulesText := extractResponseText(schoolRulesResp)
	assert.Contains(t, rulesText, "학급 생활 규칙")
	assert.Contains(t, rulesText, "바른 말 고운 말")

	// 8. /school Monday query
	schoolDayResp := reg.Execute(baseCtx("school", "월요일"))
	require.NotNil(t, schoolDayResp)
	dayText := extractResponseText(schoolDayResp)
	assert.Contains(t, dayText, "월요일 시간표")
	assert.Contains(t, dayText, "도덕")

	// 9. /alarm add
	alarmAddResp := reg.Execute(baseCtx("alarm", "add", "매주", "평일", "15:00", "정상어학원", "알림"))
	require.NotNil(t, alarmAddResp)
	assert.Contains(t, alarmAddResp.Template.Outputs[0].SimpleText.Text, "반복 알림이 등록되었습니다")

	// 10. /alarm list
	alarmListResp := reg.Execute(baseCtx("alarm", "list"))
	require.NotNil(t, alarmListResp)
	assert.Contains(t, alarmListResp.Template.Outputs[0].SimpleText.Text, "등록된 예약 알림")
	assert.Contains(t, alarmListResp.Template.Outputs[0].SimpleText.Text, "정상어학원")

	// 11. /alarm cancel all
	alarmCancelResp := reg.Execute(baseCtx("alarm", "cancel", "all"))
	require.NotNil(t, alarmCancelResp)
	assert.Contains(t, alarmCancelResp.Template.Outputs[0].SimpleText.Text, "모두 취소되었습니다")

	// 12. /data list
	dataListCtx := baseCtx("data", "list", "test-admin-token")
	dataListCtx.AdminToken = "test-admin-token"
	dataListResp := reg.Execute(dataListCtx)
	require.NotNil(t, dataListResp)
	dataListText := dataListResp.Template.Outputs[0].SimpleText.Text
	assert.Contains(t, dataListText, "등록된 데이터 카탈로그")
	assert.Contains(t, dataListText, "bus.json")
	assert.Contains(t, dataListText, "school.json")

	// 13. /data reload
	dataReloadCtx := baseCtx("data", "reload", "test-admin-token")
	dataReloadCtx.AdminToken = "test-admin-token"
	dataReloadResp := reg.Execute(dataReloadCtx)
	require.NotNil(t, dataReloadResp)
	assert.Contains(t, dataReloadResp.Template.Outputs[0].SimpleText.Text, "데이터 디렉토리 핫리로드 완료")

	// 14. /holiday
	holidayResp := reg.Execute(baseCtx("holiday"))
	require.NotNil(t, holidayResp)
	assert.Contains(t, holidayResp.Template.Outputs[0].SimpleText.Text, "오늘 일시")

	// 15. /model (list / status)
	modelResp := reg.Execute(baseCtx("model"))
	require.NotNil(t, modelResp)
	assert.Contains(t, modelResp.Template.Outputs[0].SimpleText.Text, "AI 모델 설정")
	assert.Contains(t, modelResp.Template.Outputs[0].SimpleText.Text, "시스템 기본 정책")
	assert.Contains(t, modelResp.Template.Outputs[0].SimpleText.Text, "gemini-3.8-flash")
	assert.Contains(t, modelResp.Template.Outputs[0].SimpleText.Text, "grok-4.20-non-reasoning")

	// 16. /model set grok-4.20-non-reasoning
	modelSetResp := reg.Execute(baseCtx("model", "set", "grok-4.20-non-reasoning"))
	require.NotNil(t, modelSetResp)
	assert.Contains(t, modelSetResp.Template.Outputs[0].SimpleText.Text, "AI 모델이 \"grok-4.20-non-reasoning\"(으)로 설정되었습니다")
	assert.Equal(t, "grok-4.20-non-reasoning", agent.GetDefaultModelPreferenceStore().Get("user-1"))

	// 17. /모델 (Korean alias) and reset
	modelResetResp := reg.Execute(baseCtx("모델", "reset"))
	require.NotNil(t, modelResetResp)
	assert.Contains(t, modelResetResp.Template.Outputs[0].SimpleText.Text, "시스템 기본 정책으로 초기화되었습니다")
	assert.Equal(t, "", agent.GetDefaultModelPreferenceStore().Get("user-1"))

	// 18. Unknown slash command
	unknownResp := reg.Execute(baseCtx("unknown_xyz"))
	require.NotNil(t, unknownResp)
	assert.Contains(t, unknownResp.Template.Outputs[0].SimpleText.Text, "알 수 없는 명령어입니다: /unknown_xyz")
	assert.GreaterOrEqual(t, len(unknownResp.Template.QuickReplies), 1)
}

func TestSlashCommand_EndToEndHTTPServer(t *testing.T) {
	addr := "127.0.0.1:18092"
	dataDir := t.TempDir()

	// Seed minimal bus json
	busJSON := `{
		"academy": {
			"name": "명품태권도",
			"vehicle_number": "1호차",
			"type": "등원"
		},
		"stops": [
			{
				"location": "호반베르디움",
				"schedules": { "부": "16:00" }
			}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "shuttle.json"), []byte(busJSON), 0644))

	busSvc := academy.NewService()
	require.NoError(t, busSvc.LoadFromDir(dataDir))

	var buf bytes.Buffer
	uc := NewSkillServeUseCase()
	go func() {
		_ = uc.Execute(t.Context(), SkillServeRequest{
			SkillToken: testSkillToken,
			ListenAddr: addr,
			ChannelID:  "0xc0de1ab",
			AIProvider: ai.NewMockProvider("mock-provider"),
			BusService: busSvc,
			DataDir:    dataDir,
			Out:        &buf,
		})
	}()
	waitForServer(t, "http://"+addr+"/healthz")

	// Helper for HTTP requests
	sendUtterance := func(utterance string) openbuilder.SkillResponse {
		payload := openbuilder.SkillPayload{
			UserRequest: openbuilder.UserRequest{
				Utterance: utterance,
				User: openbuilder.ChatUser{
					ID: "e2e-slash-user",
				},
			},
		}
		bodyBytes, _ := json.Marshal(payload)
		resp, err := postSkillRequest(t, "http://"+addr+"/skill", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		defer resp.Body.Close()

		var skillResp openbuilder.SkillResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&skillResp))
		return skillResp
	}

	// 1. Send /help via HTTP
	resp1 := sendUtterance("/help")
	require.NotEmpty(t, resp1.Template.Outputs)
	assert.Contains(t, extractResponseText(&resp1), "슬래시 명령어 안내")
	assert.GreaterOrEqual(t, len(resp1.Template.QuickReplies), 3)

	// 2. Send /status via HTTP
	resp2 := sendUtterance("/status")
	require.NotEmpty(t, resp2.Template.Outputs)
	assert.NotNil(t, resp2.Template.Outputs[0].BasicCard)
	assert.Contains(t, extractResponseText(&resp2), "학원 노선: 1개")

	// 3. Send /bus stop query via HTTP
	resp3 := sendUtterance("/bus 호반")
	require.NotEmpty(t, resp3.Template.Outputs)
	assert.Contains(t, extractResponseText(&resp3), "명품태권도")
	assert.Contains(t, extractResponseText(&resp3), "16:00")

	// 4. Unknown slash command falls through to the AI agent instead of dead-ending
	resp4 := sendUtterance("/invalid_test_command")
	require.NotEmpty(t, resp4.Template.Outputs)
	assert.Contains(t, extractResponseText(&resp4), "AI Mock Reply")
	assert.Contains(t, extractResponseText(&resp4), "/invalid_test_command")
}

func extractResponseText(resp *openbuilder.SkillResponse) string {
	if resp == nil || len(resp.Template.Outputs) == 0 {
		return ""
	}
	output := resp.Template.Outputs[0]
	if output.SimpleText != nil {
		return output.SimpleText.Text
	}
	if output.BasicCard != nil {
		return output.BasicCard.Description
	}
	if output.TextCard != nil {
		return output.TextCard.Description
	}
	return ""
}

func TestSlashCommand_DataAdminGate(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "bus.json"), []byte(`{"academy":{"name":"정상어학원","vehicle_number":"2호차","type":"등원"},"stops":[{"location":"우미린 2차","schedules":{"초등부":"15:00"}}]}`), 0o644))

	busSvc := academy.NewService()
	require.NoError(t, busSvc.LoadFromDir(tempDir))

	execute := func(adminToken string, args ...string) *openbuilder.SkillResponse {
		registry := NewDefaultSlashCommandRegistry()
		return registry.Execute(&CommandContext{
			Ctx:        t.Context(),
			RawText:    "/data " + strings.Join(args, " "),
			Command:    "data",
			Args:       args,
			UserID:     "gate-user",
			DataDir:    tempDir,
			AdminToken: adminToken,
			BusSvc:     busSvc,
			Registry:   nil,
		})
	}

	// Missing configuration must deny access just like the HTTP admin API.
	resp := execute("", "list")
	require.NotNil(t, resp)
	assert.Contains(t, extractResponseText(resp), "관리자 인증이 필요합니다")

	// Configured token without/wrong argument is denied.
	resp = execute("secret-token", "list")
	require.NotNil(t, resp)
	assert.Contains(t, extractResponseText(resp), "관리자 인증이 필요합니다")
	resp = execute("secret-token", "list", "wrong-token")
	require.NotNil(t, resp)
	assert.Contains(t, extractResponseText(resp), "관리자 인증이 필요합니다")

	// Correct token as last argument grants access and is stripped from args.
	resp = execute("secret-token", "list", "secret-token")
	require.NotNil(t, resp)
	assert.Contains(t, extractResponseText(resp), "데이터 카탈로그")

	// Reload is gated identically.
	resp = execute("secret-token", "reload")
	require.NotNil(t, resp)
	assert.Contains(t, extractResponseText(resp), "관리자 인증이 필요합니다")
	resp = execute("secret-token", "reload", "secret-token")
	require.NotNil(t, resp)
	assert.Contains(t, extractResponseText(resp), "핫리로드 완료")
}

func TestSlashCommand_Model(t *testing.T) {
	prefStore := agent.GetDefaultModelPreferenceStore()
	testUser := "test-model-user"
	prefStore.Reset(testUser)

	reg := GetDefaultSlashCommandRegistry()

	// 1. Initial /model list
	resp := reg.Execute(&CommandContext{
		Command: "model",
		UserID:  testUser,
	})
	require.NotNil(t, resp)
	txt := extractResponseText(resp)
	assert.Contains(t, txt, "AI 모델 설정 및 조회")
	assert.Contains(t, txt, "gemini-3.8-flash")
	assert.Contains(t, txt, "grok-4.20-non-reasoning")
	assert.Contains(t, txt, "grok-4.3")
	assert.Contains(t, txt, "기본 정책 적용 중")

	// 2. /model set grok-4.20-non-reasoning
	resp = reg.Execute(&CommandContext{
		Command: "model",
		Args:    []string{"set", "grok-4.20-non-reasoning"},
		UserID:  testUser,
	})
	require.NotNil(t, resp)
	assert.Contains(t, extractResponseText(resp), "grok-4.20-non-reasoning")
	assert.Equal(t, "grok-4.20-non-reasoning", prefStore.Get(testUser))

	// 3. /model list reflects custom user choice
	resp = reg.Execute(&CommandContext{
		Command: "model",
		UserID:  testUser,
	})
	require.NotNil(t, resp)
	assert.Contains(t, extractResponseText(resp), "사용자 지정")

	// 4. /모델 alias works in Korean
	resp = reg.Execute(&CommandContext{
		Command: "모델",
		Args:    []string{"설정", "gemini-3.8-flash"},
		UserID:  testUser,
	})
	require.NotNil(t, resp)
	assert.Contains(t, extractResponseText(resp), "gemini-3.8-flash")
	assert.Equal(t, "gemini-3.8-flash", prefStore.Get(testUser))

	// 5. /model reset
	resp = reg.Execute(&CommandContext{
		Command: "model",
		Args:    []string{"reset"},
		UserID:  testUser,
	})
	require.NotNil(t, resp)
	assert.Contains(t, extractResponseText(resp), "기본 정책으로 초기화")
	assert.Equal(t, "", prefStore.Get(testUser))

	// 6. Invalid model
	resp = reg.Execute(&CommandContext{
		Command: "model",
		Args:    []string{"set", "non-existent-model"},
		UserID:  testUser,
	})
	require.NotNil(t, resp)
	assert.Contains(t, extractResponseText(resp), "지원하지 않는 모델")
}
