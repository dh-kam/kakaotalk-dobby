package skill

import (
	"context"
	"crypto/subtle"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/academy"
	"github.com/dh-kam/kakaotalk-dobby/pkg/agent"
	"github.com/dh-kam/kakaotalk-dobby/pkg/holidays"
	"github.com/dh-kam/kakaotalk-dobby/pkg/openbuilder"
	"github.com/dh-kam/kakaotalk-dobby/pkg/scheduler"
	"github.com/dh-kam/kakaotalk-dobby/pkg/school"
	"github.com/samber/lo"
)

// CommandContext encapsulates metadata and dependencies required to execute a slash command.
type CommandContext struct {
	Ctx          context.Context
	RawText      string
	Command      string
	Args         []string
	UserID       string
	ChannelID    string
	DataDir      string
	AdminToken   string
	BusSvc       *academy.Service
	SchoolSvc    *school.Service
	SchedEngine  *scheduler.Engine
	SessionStore agent.SessionStore
	Registry     *SlashCommandRegistry
}

// verifyAdminToken reports whether the command carries the configured admin token.
// The token must be supplied as the final argument; an empty AdminToken keeps the
// command open, mirroring the /api/data/* dev mode.
func (cc *CommandContext) verifyAdminToken() bool {
	if cc.AdminToken == "" {
		return true
	}
	if len(cc.Args) == 0 {
		return false
	}
	given := strings.TrimSpace(cc.Args[len(cc.Args)-1])
	return subtle.ConstantTimeCompare([]byte(given), []byte(cc.AdminToken)) == 1
}

// popAdminToken removes the trailing admin token argument after successful verification.
func (cc *CommandContext) popAdminToken() {
	if cc.AdminToken != "" && len(cc.Args) > 0 {
		cc.Args = cc.Args[:len(cc.Args)-1]
	}
}

// CommandHandler defines the function signature for slash command execution.
type CommandHandler func(cc *CommandContext) *openbuilder.SkillResponse

// CommandSpec defines metadata, syntax, aliases, and handler for a slash command.
type CommandSpec struct {
	Name        string
	Aliases     []string
	Description string
	Usage       string
	Category    string
	Handler     CommandHandler
}

// SlashCommandRegistry manages registered slash commands and routes invocations.
type SlashCommandRegistry struct {
	mu       sync.RWMutex
	commands map[string]CommandSpec
	aliases  map[string]string
}

// NewSlashCommandRegistry creates an empty slash command registry.
func NewSlashCommandRegistry() *SlashCommandRegistry {
	return &SlashCommandRegistry{
		commands: make(map[string]CommandSpec),
		aliases:  make(map[string]string),
	}
}

// Register registers a command specification into the registry.
func (r *SlashCommandRegistry) Register(spec CommandSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()

	lowerName := strings.ToLower(strings.TrimSpace(spec.Name))
	r.commands[lowerName] = spec

	for _, alias := range spec.Aliases {
		lowerAlias := strings.ToLower(strings.TrimSpace(alias))
		if lowerAlias != "" {
			r.aliases[lowerAlias] = lowerName
		}
	}
}

// Find resolves a command by its canonical name or alias.
func (r *SlashCommandRegistry) Find(name string) (CommandSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lower := strings.ToLower(strings.TrimSpace(name))
	if canonical, ok := r.aliases[lower]; ok {
		lower = canonical
	}

	spec, ok := r.commands[lower]
	return spec, ok
}

// List returns all registered command specifications.
func (r *SlashCommandRegistry) List() []CommandSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return lo.Values(r.commands)
}

// Execute parses command arguments and dispatches execution to the registered handler.
func (r *SlashCommandRegistry) Execute(cc *CommandContext) *openbuilder.SkillResponse {
	if cc == nil {
		return nil
	}

	if len([]rune(cc.RawText)) > 2000 {
		return openbuilder.NewSimpleTextResponse("⚠️ 입력한 명령어 길이가 제한(2,000자)을 초과했습니다.")
	}

	spec, found := r.Find(cc.Command)
	if !found {
		return r.handleUnknown(cc)
	}

	cc.Registry = r
	return spec.Handler(cc)
}

func (r *SlashCommandRegistry) handleUnknown(cc *CommandContext) *openbuilder.SkillResponse {
	text := fmt.Sprintf("⚠️ 알 수 없는 명령어입니다: /%s\n\n💡 '/help' 명령어로 사용 가능한 전체 명령어 목록을 확인하실 수 있습니다.", cc.Command)
	resp := openbuilder.NewSimpleTextResponse(text)
	resp.AddQuickReply("명령어 도움말", "/help")
	resp.AddQuickReply("서버 상태", "/status")
	resp.AddQuickReply("학원 버스", "/bus")
	resp.AddQuickReply("학교 시간표", "/school")
	return resp
}

// parseSlashCommand extracts command name and arguments from an utterance starting with slash.
func parseSlashCommand(utterance string) (string, []string) {
	trimmed := strings.TrimSpace(utterance)
	if !strings.HasPrefix(trimmed, "/") {
		return "", nil
	}

	trimmed = strings.TrimLeft(trimmed, "/")
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "help", []string{}
	}

	return strings.ToLower(fields[0]), fields[1:]
}

var (
	defaultSlashRegistry     *SlashCommandRegistry
	defaultSlashRegistryOnce sync.Once
)

// GetDefaultSlashCommandRegistry returns the lazily initialized package default registry.
func GetDefaultSlashCommandRegistry() *SlashCommandRegistry {
	defaultSlashRegistryOnce.Do(func() {
		defaultSlashRegistry = NewDefaultSlashCommandRegistry()
	})
	return defaultSlashRegistry
}

// NewDefaultSlashCommandRegistry initializes the default registry with core commands.
func NewDefaultSlashCommandRegistry() *SlashCommandRegistry {
	r := NewSlashCommandRegistry()

	r.Register(CommandSpec{
		Name:        "help",
		Aliases:     []string{"?", "commands", "도움말"},
		Description: "전체 슬래시 명령어 목록 및 사용법 안내",
		Usage:       "/help",
		Category:    "일반",
		Handler:     cmdHelpHandler,
	})

	r.Register(CommandSpec{
		Name:        "reset",
		Aliases:     []string{"clear", "초기화", "리셋"},
		Description: "현재 대화 세션 기억 즉시 초기화",
		Usage:       "/reset",
		Category:    "일반",
		Handler:     cmdResetHandler,
	})

	r.Register(CommandSpec{
		Name:        "status",
		Aliases:     []string{"stat", "info", "상태"},
		Description: "서버 리소스 및 서비스 운영 상태 확인",
		Usage:       "/status",
		Category:    "일반",
		Handler:     cmdStatusHandler,
	})

	r.Register(CommandSpec{
		Name:        "ping",
		Aliases:     []string{"핑"},
		Description: "서버 통신 및 응답 상태 확인",
		Usage:       "/ping",
		Category:    "일반",
		Handler:     cmdPingHandler,
	})

	r.Register(CommandSpec{
		Name:        "bus",
		Aliases:     []string{"버스", "셔틀"},
		Description: "학원 셔틀버스 시간표 즉각 조회",
		Usage:       "/bus [학원명] [정류장]",
		Category:    "일정/교통",
		Handler:     cmdBusHandler,
	})

	r.Register(CommandSpec{
		Name:        "school",
		Aliases:     []string{"학교", "시간표"},
		Description: "초등학교 요일별 시간표 및 학급 생활 수칙 조회",
		Usage:       "/school [요일|규칙]",
		Category:    "일정/교통",
		Handler:     cmdSchoolHandler,
	})

	r.Register(CommandSpec{
		Name:        "alarm",
		Aliases:     []string{"sched", "schedule", "알림", "예약"},
		Description: "예약 알림 목록 조회, 취소 및 등록",
		Usage:       "/alarm [list|cancel <id|all>|add <내용>]",
		Category:    "알림/스케줄",
		Handler:     cmdAlarmHandler,
	})

	r.Register(CommandSpec{
		Name:        "data",
		Aliases:     []string{"catalog", "데이터"},
		Description: "데이터 디렉토리 카탈로그 확인 및 런타임 핫리로드",
		Usage:       "/data [list|reload] [관리자토큰]",
		Category:    "관리/운영",
		Handler:     cmdDataHandler,
	})

	r.Register(CommandSpec{
		Name:        "holiday",
		Aliases:     []string{"holidays", "휴일", "공휴일"},
		Description: "오늘 및 지정 일자의 공휴일/기념일 정보 확인",
		Usage:       "/holiday",
		Category:    "일정/교통",
		Handler:     cmdHolidayHandler,
	})

	r.Register(CommandSpec{
		Name:        "model",
		Aliases:     []string{"llm", "모델"},
		Description: "현재 AI 모델 확인 및 변경 (Gemini/Grok 등)",
		Usage:       "/model [list|set <모델명>|reset]",
		Category:    "일반",
		Handler:     cmdModelHandler,
	})

	return r
}

func cmdHelpHandler(cc *CommandContext) *openbuilder.SkillResponse {
	var sb strings.Builder
	sb.WriteString("🤖 KakaoBot Dobby 슬래시 명령어 안내:\n\n")

	reg := cc.Registry
	if reg == nil {
		reg = GetDefaultSlashCommandRegistry()
	}
	specs := reg.List()
	categories := []string{"일반", "일정/교통", "알림/스케줄", "관리/운영"}
	grouped := lo.GroupBy(specs, func(s CommandSpec) string {
		return s.Category
	})

	categoryIcons := map[string]string{
		"일반":     "📌",
		"일정/교통":  "🚌",
		"알림/스케줄": "⏰",
		"관리/운영":  "📁",
	}

	for _, cat := range categories {
		items := grouped[cat]
		if len(items) == 0 {
			continue
		}
		icon := categoryIcons[cat]
		sb.WriteString(fmt.Sprintf("%s %s 명령어:\n", icon, cat))
		for _, item := range items {
			sb.WriteString(fmt.Sprintf("• %s : %s\n", item.Usage, item.Description))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("💡 채팅창에 명령어를 직접 입력하거나 아래 버튼을 터치해 보세요.")

	resp := openbuilder.NewSimpleTextResponse(strings.TrimSpace(sb.String()))
	resp.AddQuickReply("서버 상태", "/status")
	resp.AddQuickReply("모델 설정", "/model")
	resp.AddQuickReply("대화 리셋", "/reset")
	resp.AddQuickReply("학원 버스", "/bus")
	resp.AddQuickReply("학교 시간표", "/school")
	resp.AddQuickReply("알림 목록", "/alarm list")
	return resp
}

func cmdResetHandler(cc *CommandContext) *openbuilder.SkillResponse {
	if cc.SessionStore != nil {
		cc.SessionStore.Clear(cc.UserID)
	}

	resp := openbuilder.NewSimpleTextResponse("🔄 이전 대화 기억을 초기화했습니다. 무엇이든 새로 물어보세요!")
	resp.AddQuickReply("명령어 도움말", "/help")
	resp.AddQuickReply("학원 버스", "/bus")
	resp.AddQuickReply("학교 시간표", "/school")
	return resp
}

func cmdStatusHandler(cc *CommandContext) *openbuilder.SkillResponse {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	busCount := 0
	if cc.BusSvc != nil {
		busCount = len(cc.BusSvc.ListAcademies())
	}

	schoolSummary := "없음"
	if cc.SchoolSvc != nil && cc.SchoolSvc.GetSummary() != "" {
		schoolSummary = cc.SchoolSvc.GetSummary()
	}

	myAlarms := 0
	if cc.SchedEngine != nil {
		myAlarms = lo.CountBy(cc.SchedEngine.ListJobs(strings.TrimSpace(cc.UserID)), func(j *scheduler.Job) bool {
			return j.Status == scheduler.JobStatusActive
		})
	}

	desc := fmt.Sprintf("OS: %s/%s\nGo: %s\nGoroutines: %d\nAlloc: %.2f MB\n학원 노선: %d개 등록\n학교 시간표: %s\n내 예약 알림: %d건 활성",
		runtime.GOOS, runtime.GOARCH, runtime.Version(), runtime.NumGoroutine(), float64(m.Alloc)/(1024*1024), busCount, schoolSummary, myAlarms)

	resp := openbuilder.NewBasicCardResponse(
		"🖥️ 서버 상태 보고서",
		desc,
		"",
		openbuilder.NewMessageButton("다시 조회", "/status"),
		openbuilder.NewMessageButton("도움말", "/help"),
	)
	resp.AddQuickReply("명령어 도움말", "/help")
	resp.AddQuickReply("데이터 확인", "/data list")
	return resp
}

func cmdPingHandler(_ *CommandContext) *openbuilder.SkillResponse {
	resp := openbuilder.NewSimpleTextResponse("🏓 퐁! (Pong) 서버가 정상적으로 응답하고 있습니다.")
	resp.AddQuickReply("서버 상태", "/status")
	resp.AddQuickReply("명령어 도움말", "/help")
	return resp
}

func cmdBusHandler(cc *CommandContext) *openbuilder.SkillResponse {
	if cc.BusSvc == nil {
		resp := openbuilder.NewSimpleTextResponse("⚠️ 학원 버스 서비스가 준비되지 않았습니다.")
		resp.AddQuickReply("명령어 도움말", "/help")
		return resp
	}

	if len(cc.Args) == 0 {
		academies := cc.BusSvc.ListAcademies()
		var sb strings.Builder
		sb.WriteString("🚌 등록된 학원 버스 노선 목록:\n\n")
		if len(academies) == 0 {
			sb.WriteString("• 현재 등록된 학원 버스 시간표가 없습니다.\n")
		} else {
			for _, ac := range academies {
				sb.WriteString(fmt.Sprintf("• %s\n", ac))
			}
		}

		sb.WriteString("\n💡 사용법:\n")
		sb.WriteString("• /bus [정류장/위치] (예: /bus 우미린)\n")
		sb.WriteString("• /bus [학원명] [정류장] (예: /bus 정상 2호차)\n")

		resp := openbuilder.NewSimpleTextResponse(strings.TrimSpace(sb.String()))
		resp.AddQuickReply("우미린 버스", "/bus 우미린")
		resp.AddQuickReply("해마루 버스", "/bus 해마루")
		resp.AddQuickReply("명령어 도움말", "/help")
		return resp
	}

	query := strings.Join(cc.Args, " ")
	resp := handleBusScheduleFastPath(cc.BusSvc, query)
	if resp != nil {
		return resp
	}

	notFoundMsg := fmt.Sprintf("🔍 '%s' 관련 버스 시간표를 찾을 수 없습니다.\n\n등록된 정류장(우미린, 해마루초, 호반, 이편한, 중흥, 양포 등) 또는 학원명을 확인해 주세요.", query)
	resp = openbuilder.NewSimpleTextResponse(notFoundMsg)
	resp.AddQuickReply("버스 노선 목록", "/bus")
	resp.AddQuickReply("명령어 도움말", "/help")
	return resp
}

func cmdSchoolHandler(cc *CommandContext) *openbuilder.SkillResponse {
	if cc.SchoolSvc == nil {
		resp := openbuilder.NewSimpleTextResponse("⚠️ 학교 시간표 서비스가 준비되지 않았습니다.")
		resp.AddQuickReply("명령어 도움말", "/help")
		return resp
	}

	if len(cc.Args) == 0 {
		resp := handleSchoolTimetableFastPath(cc.SchoolSvc, "오늘")
		if resp != nil {
			return resp
		}

		msg := fmt.Sprintf("🏫 %s\n\n오늘은 주말이거나 시간표가 등록되어 있지 않습니다.\n원하시는 요일을 입력해 확인해 보세요! (예: /school 월요일)", cc.SchoolSvc.GetSummary())
		resp = openbuilder.NewSimpleTextResponse(msg)
		resp.AddQuickReply("월요일 시간표", "/school 월요일")
		resp.AddQuickReply("교실 수칙", "/school 규칙")
		resp.AddQuickReply("명령어 도움말", "/help")
		return resp
	}

	query := strings.Join(cc.Args, " ")
	resp := handleSchoolTimetableFastPath(cc.SchoolSvc, query)
	if resp != nil {
		return resp
	}

	notFoundMsg := fmt.Sprintf("🏫 '%s'에 해당하는 시간표 또는 규칙을 찾을 수 없습니다.\n\n💡 사용법: /school [월/화/수/목/금] 또는 /school 규칙", query)
	resp = openbuilder.NewSimpleTextResponse(notFoundMsg)
	resp.AddQuickReply("월요일 시간표", "/school 월요일")
	resp.AddQuickReply("교실 수칙", "/school 규칙")
	resp.AddQuickReply("명령어 도움말", "/help")
	return resp
}

func cmdAlarmHandler(cc *CommandContext) *openbuilder.SkillResponse {
	if cc.SchedEngine == nil {
		resp := openbuilder.NewSimpleTextResponse("⚠️ 스케줄러 엔진이 활성화되지 않았습니다.")
		resp.AddQuickReply("명령어 도움말", "/help")
		return resp
	}

	cleanUserID := strings.TrimSpace(cc.UserID)
	if cleanUserID == "" {
		resp := openbuilder.NewSimpleTextResponse("⚠️ 유효한 사용자 식별 정보가 없어 알림 서비스를 이용할 수 없습니다.")
		resp.AddQuickReply("명령어 도움말", "/help")
		return resp
	}

	action := "list"
	if len(cc.Args) > 0 {
		action = strings.ToLower(cc.Args[0])
	}

	switch action {
	case "list", "목록", "조회":
		jobs := cc.SchedEngine.ListJobs(cleanUserID)
		if len(jobs) == 0 {
			resp := openbuilder.NewSimpleTextResponse("📋 현재 등록된 예약 알림이 없습니다.\n\n💡 알림 등록 예시:\n• /alarm add 매주 평일 15:00 정상어학원\n• /alarm add 10분 뒤 라면")
			resp.AddQuickReply("명령어 도움말", "/help")
			return resp
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📋 등록된 예약 알림 (총 %d건):\n\n", len(jobs)))
		displayLimit := 10
		for i, j := range jobs {
			if i >= displayLimit {
				sb.WriteString(fmt.Sprintf("... 외 %d건 생략\n\n", len(jobs)-displayLimit))
				break
			}
			statusIcon := "🟢"
			if j.Status == scheduler.JobStatusCompleted {
				statusIcon = "✅"
			} else if j.Status == scheduler.JobStatusCancelled {
				statusIcon = "❌"
			}
			sb.WriteString(fmt.Sprintf("%d. %s %s [ID: %s]\n", i+1, statusIcon, j.Title, j.ID))
			if j.Type == scheduler.ScheduleTypeOnce {
				sb.WriteString(fmt.Sprintf("   • 실행: %s\n", j.ExecuteAt.Format("01/02 15:04")))
			} else {
				sb.WriteString(fmt.Sprintf("   • 주기: %s\n", j.CronExpr))
			}
			sb.WriteString(fmt.Sprintf("   • 내용: %s\n\n", j.Message))
		}
		sb.WriteString("💡 취소: /alarm cancel <ID> 또는 /alarm cancel all")

		resp := openbuilder.NewSimpleTextResponse(strings.TrimSpace(sb.String()))
		resp.AddQuickReply("전체 취소", "/alarm cancel all")
		resp.AddQuickReply("명령어 도움말", "/help")
		return resp

	case "cancel", "취소", "삭제":
		if len(cc.Args) < 2 {
			resp := openbuilder.NewSimpleTextResponse("⚠️ 취소할 알림 ID를 입력해 주세요.\n\n사용법:\n• /alarm cancel <job_id>\n• /alarm cancel all (전체 취소)")
			resp.AddQuickReply("알림 목록", "/alarm list")
			return resp
		}

		target := strings.ToLower(cc.Args[1])
		if target == "all" || target == "전체" {
			jobs := cc.SchedEngine.ListJobs(cc.UserID)
			cancelledCount := 0
			for _, j := range jobs {
				if j.Status == scheduler.JobStatusActive {
					if err := cc.SchedEngine.CancelJob(j.ID, cc.UserID); err == nil {
						cancelledCount++
					}
				}
			}
			resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf("✅ 활성 상태의 알림 %d건이 모두 취소되었습니다.", cancelledCount))
			resp.AddQuickReply("알림 목록", "/alarm list")
			resp.AddQuickReply("명령어 도움말", "/help")
			return resp
		}

		if err := cc.SchedEngine.CancelJob(cc.Args[1], cc.UserID); err != nil {
			resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf("⚠️ 알림 취소 실패: %v\n\n올바른 알림 ID인지 확인해 주세요.", err))
			resp.AddQuickReply("알림 목록", "/alarm list")
			return resp
		}

		resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf("✅ 알림 ID %s 가 성공적으로 취소되었습니다.", cc.Args[1]))
		resp.AddQuickReply("알림 목록", "/alarm list")
		resp.AddQuickReply("명령어 도움말", "/help")
		return resp

	case "add", "등록", "생성":
		if len(cc.Args) < 2 {
			resp := openbuilder.NewSimpleTextResponse("💡 알림 등록 예시:\n• /alarm add 매주 평일 15:00 정상어학원\n• /alarm add 10분 뒤 라면 끓이기")
			resp.AddQuickReply("알림 목록", "/alarm list")
			return resp
		}

		rem := strings.Join(cc.Args[1:], " ")
		if resp := handleScheduleFastPath(cc.SchedEngine, rem, cc.UserID); resp != nil {
			return resp
		}

		resp := openbuilder.NewSimpleTextResponse("⚠️ 알림 형식을 인식하지 못했습니다.\n\n예: /alarm add 매주 평일 15:00 정상어학원 알림 등록해줘")
		resp.AddQuickReply("알림 목록", "/alarm list")
		return resp

	default:
		resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf("⚠️ 지원하지 않는 알림 하위 명령어입니다: %s\n\n💡 사용법: /alarm [list|cancel <id>|add <내용>]", action))
		resp.AddQuickReply("알림 목록", "/alarm list")
		resp.AddQuickReply("명령어 도움말", "/help")
		return resp
	}
}

func cmdDataHandler(cc *CommandContext) *openbuilder.SkillResponse {
	if !cc.verifyAdminToken() {
		resp := openbuilder.NewSimpleTextResponse("⚠️ 관리자 인증이 필요합니다. 명령 끝에 관리자 토큰을 붙여 주세요.\n\n예: /data list <관리자토큰>")
		return resp
	}
	cc.popAdminToken()

	action := "list"
	if len(cc.Args) > 0 {
		action = strings.ToLower(cc.Args[0])
	}

	switch action {
	case "list", "목록", "조회":
		var files []string
		if cc.DataDir != "" {
			entries, err := os.ReadDir(cc.DataDir)
			if err == nil {
				files = lo.FilterMap(entries, func(e os.DirEntry, _ int) (string, bool) {
					if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
						return e.Name(), true
					}
					return "", false
				})
			}
		}

		var sb strings.Builder
		sb.WriteString("📁 등록된 데이터 카탈로그 정보:\n\n")
		sb.WriteString(fmt.Sprintf("• 데이터 디렉토리: %s\n", cc.DataDir))
		sb.WriteString(fmt.Sprintf("• JSON 파일 목록 (%d개):\n", len(files)))
		displayLimit := 15
		for i, f := range files {
			if i >= displayLimit {
				sb.WriteString(fmt.Sprintf("  ... 외 %d개 파일 생략\n", len(files)-displayLimit))
				break
			}
			sb.WriteString(fmt.Sprintf("  - %s\n", f))
		}

		busSummary := "미등록"
		if cc.BusSvc != nil {
			academies := cc.BusSvc.ListAcademies()
			busSummary = fmt.Sprintf("%d개 노선 (%s)", len(academies), strings.Join(academies, ", "))
		}
		sb.WriteString(fmt.Sprintf("\n• 학원 셔틀: %s\n", busSummary))

		schoolSummary := "미등록"
		if cc.SchoolSvc != nil && cc.SchoolSvc.GetSummary() != "" {
			schoolSummary = cc.SchoolSvc.GetSummary()
		}
		sb.WriteString(fmt.Sprintf("• 학교 시간표: %s\n", schoolSummary))

		resp := openbuilder.NewSimpleTextResponse(strings.TrimSpace(sb.String()))
		resp.AddQuickReply("데이터 리로드", "/data reload")
		resp.AddQuickReply("서버 상태", "/status")
		resp.AddQuickReply("명령어 도움말", "/help")
		return resp

	case "reload", "리로드", "새로고침":
		if cc.DataDir == "" {
			resp := openbuilder.NewSimpleTextResponse("⚠️ 데이터 디렉토리가 설정되어 있지 않아 리로드할 수 없습니다.")
			return resp
		}

		var busErr error
		if cc.BusSvc != nil {
			busErr = cc.BusSvc.ReloadFromDir(cc.DataDir)
		}
		var schoolErr error
		if cc.SchoolSvc != nil {
			schoolErr = cc.SchoolSvc.ReloadFromDir(cc.DataDir)
		}

		var sb strings.Builder
		sb.WriteString("🔄 데이터 디렉토리 핫리로드 완료!\n\n")
		sb.WriteString(fmt.Sprintf("• 경로: %s\n", filepath.Clean(cc.DataDir)))

		if busErr != nil {
			sb.WriteString(fmt.Sprintf("• 학원 버스 리로드 오류: %v\n", busErr))
		} else if cc.BusSvc != nil {
			sb.WriteString(fmt.Sprintf("• 학원 노선: %d개 활성화\n", len(cc.BusSvc.ListAcademies())))
		}

		if schoolErr != nil {
			sb.WriteString(fmt.Sprintf("• 학교 시간표 리로드 오류: %v\n", schoolErr))
		} else if cc.SchoolSvc != nil {
			sb.WriteString(fmt.Sprintf("• 학교 시간표: %s\n", cc.SchoolSvc.GetSummary()))
		}

		resp := openbuilder.NewSimpleTextResponse(strings.TrimSpace(sb.String()))
		resp.AddQuickReply("데이터 목록", "/data list")
		resp.AddQuickReply("학원 버스", "/bus")
		resp.AddQuickReply("학교 시간표", "/school")
		return resp

	default:
		resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf("⚠️ 지원하지 않는 데이터 명령어입니다: %s\n\n사용법: /data [list|reload]", action))
		resp.AddQuickReply("데이터 목록", "/data list")
		resp.AddQuickReply("명령어 도움말", "/help")
		return resp
	}
}

func cmdHolidayHandler(_ *CommandContext) *openbuilder.SkillResponse {
	now := time.Now().In(holidays.GetKSTLocation())
	info := holidays.CheckDate(now)

	statusDesc := info.Description
	if statusDesc == "" {
		statusDesc = "특별한 공휴일이나 기념일이 아닙니다."
	}

	text := fmt.Sprintf("⏰ 오늘 일시: %s (%s) %s\n📅 정보: %s",
		info.Date, info.Weekday, now.Format("15:04:05 KST"), statusDesc)

	resp := openbuilder.NewSimpleTextResponse(text)
	resp.AddQuickReply("서버 상태", "/status")
	resp.AddQuickReply("학교 시간표", "/school")
	resp.AddQuickReply("명령어 도움말", "/help")
	return resp
}

func cmdModelHandler(cc *CommandContext) *openbuilder.SkillResponse {
	prefStore := agent.GetDefaultModelPreferenceStore()
	userPref := prefStore.Get(cc.UserID)
	policyModel := agent.ResolvePolicyDefaultModel(time.Now(), agent.VertexConfig{})

	currentActive := userPref
	isCustom := true
	if currentActive == "" {
		currentActive = policyModel
		isCustom = false
	}

	if len(cc.Args) == 0 || cc.Args[0] == "list" || cc.Args[0] == "목록" || cc.Args[0] == "조회" {
		var sb strings.Builder
		sb.WriteString("🤖 AI 모델 설정 및 조회 (/model):\n\n")
		if isCustom {
			sb.WriteString(fmt.Sprintf("• 현재 적용 모델: %s (사용자 지정 🎯)\n", currentActive))
		} else {
			sb.WriteString(fmt.Sprintf("• 현재 적용 모델: %s (기본 정책 적용 중 ⚡)\n", currentActive))
		}

		sb.WriteString("\n📅 시스템 기본 정책 (가성비 최적화):\n")
		sb.WriteString("• ~2026.12.31: 최신 Gemini Flash (현재: gemini-3.8-flash)\n")
		sb.WriteString("• 2027.01.01~: Grok Non-Reasoning (grok-4.20-non-reasoning)\n\n")

		sb.WriteString("📋 선택 가능한 모델 목록:\n")
		for i, m := range agent.AvailableModels {
			sb.WriteString(fmt.Sprintf("%d. %s (%s)\n   └ %s\n", i+1, m.Name, m.ID, m.Description))
		}
		sb.WriteString("\n💡 명령어 사용법:\n")
		sb.WriteString("• /model set <모델명> : 해당 모델로 고정\n")
		sb.WriteString("• /model reset : 시스템 기본 정책으로 복원\n")
		sb.WriteString("또는 아래 버튼을 터치하여 바로 변경할 수 있습니다.")

		resp := openbuilder.NewSimpleTextResponse(strings.TrimSpace(sb.String()))
		resp.AddQuickReply("Gemini 3.8 Flash", "/model set gemini-3.8-flash")
		resp.AddQuickReply("Grok 4.20", "/model set grok-4.20-non-reasoning")
		resp.AddQuickReply("Grok 4.3", "/model set grok-4.3")
		resp.AddQuickReply("기본 정책(자동)", "/model reset")
		resp.AddQuickReply("명령어 도움말", "/help")
		return resp
	}

	sub := strings.ToLower(cc.Args[0])
	switch sub {
	case "reset", "초기화", "기본", "auto", "자동":
		prefStore.Reset(cc.UserID)
		resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf("🔄 AI 모델이 시스템 기본 정책으로 초기화되었습니다.\n\n• 현재 활성 모델: %s\n(2026년 말까지 Gemini Flash, 2027년부터 Grok 4.20 자동 선택)", policyModel))
		resp.AddQuickReply("모델 설정", "/model")
		resp.AddQuickReply("서버 상태", "/status")
		return resp

	case "set", "설정", "선택", "select":
		if len(cc.Args) < 2 {
			resp := openbuilder.NewSimpleTextResponse("⚠️ 변경할 모델명을 입력해 주세요.\n\n예: /model set gemini-3.8-flash\n/model set grok-4.20-non-reasoning\n/model set grok-4.3")
			resp.AddQuickReply("Gemini 3.8 Flash", "/model set gemini-3.8-flash")
			resp.AddQuickReply("Grok 4.20", "/model set grok-4.20-non-reasoning")
			resp.AddQuickReply("Grok 4.3", "/model set grok-4.3")
			return resp
		}
		rawTarget := cc.Args[1]
		canonical, ok := agent.NormalizeModelAlias(rawTarget)
		if !ok {
			resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf("⚠️ 지원하지 않는 모델입니다: %q\n\n선택 가능: gemini-3.8-flash, grok-4.20-non-reasoning, grok-4.3, auto", rawTarget))
			resp.AddQuickReply("Gemini 3.8 Flash", "/model set gemini-3.8-flash")
			resp.AddQuickReply("Grok 4.20", "/model set grok-4.20-non-reasoning")
			resp.AddQuickReply("Grok 4.3", "/model set grok-4.3")
			return resp
		}

		if canonical == "auto" {
			prefStore.Reset(cc.UserID)
			resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf("🔄 AI 모델이 시스템 기본 정책으로 초기화되었습니다.\n\n• 현재 활성 모델: %s", policyModel))
			resp.AddQuickReply("모델 설정", "/model")
			return resp
		}

		prefStore.Set(cc.UserID, canonical)
		resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf("✅ AI 모델이 %q(으)로 설정되었습니다!\n\n이후 전송하시는 모든 질문에 해당 모델이 적용됩니다.", canonical))
		resp.AddQuickReply("기본 정책으로 복원", "/model reset")
		resp.AddQuickReply("모델 목록", "/model")
		return resp

	default:
		canonical, ok := agent.NormalizeModelAlias(sub)
		if ok {
			if canonical == "auto" {
				prefStore.Reset(cc.UserID)
				resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf("🔄 AI 모델이 시스템 기본 정책으로 초기화되었습니다.\n\n• 현재 활성 모델: %s", policyModel))
				resp.AddQuickReply("모델 설정", "/model")
				return resp
			}
			prefStore.Set(cc.UserID, canonical)
			resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf("✅ AI 모델이 %q(으)로 설정되었습니다!\n\n이후 전송하시는 모든 질문에 해당 모델이 적용됩니다.", canonical))
			resp.AddQuickReply("기본 정책으로 복원", "/model reset")
			resp.AddQuickReply("모델 목록", "/model")
			return resp
		}

		resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf("⚠️ 알 수 없는 모델 하위 명령어입니다: %q\n\n사용법: /model [list|set <모델명>|reset]", sub))
		resp.AddQuickReply("모델 목록", "/model")
		resp.AddQuickReply("명령어 도움말", "/help")
		return resp
	}
}
