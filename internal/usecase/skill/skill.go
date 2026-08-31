package skill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/academy"
	"github.com/dh-kam/kakaotalk-dobby/pkg/agent"
	"github.com/dh-kam/kakaotalk-dobby/pkg/ai"
	"github.com/dh-kam/kakaotalk-dobby/pkg/holidays"
	"github.com/dh-kam/kakaotalk-dobby/pkg/openbuilder"
	"github.com/dh-kam/kakaotalk-dobby/pkg/scheduler"
)

// SkillServeRequest holds options for the OpenBuilder Skill Server.
type SkillServeRequest struct {
	ListenAddr   string
	ChannelID    string
	AIProvider   ai.Provider
	Agent        agent.Agent
	BusService   *academy.Service
	Scheduler    *scheduler.Engine
	SystemPrompt string
	Out          io.Writer
}

// SkillServeUseCase runs the HTTP skill webhook server with AI integration.
type SkillServeUseCase struct{}

func NewSkillServeUseCase() *SkillServeUseCase {
	return &SkillServeUseCase{}
}

func (uc *SkillServeUseCase) Execute(ctx context.Context, req SkillServeRequest) error {
	out := req.Out
	if out == nil {
		out = os.Stdout
	}
	if req.ListenAddr == "" {
		req.ListenAddr = ":8080"
	}
	if req.ChannelID == "" {
		req.ChannelID = "0xc0de1ab"
	}
	if req.AIProvider == nil {
		req.AIProvider = ai.NewMockProvider("default")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var payload openbuilder.SkillPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
			return
		}

		utterance := strings.TrimSpace(payload.UserRequest.Utterance)
		if utterance == "" {
			utterance = payload.Action.Params["utterance"]
		}

		respPayload := processUtterance(r.Context(), utterance, req.ChannelID, req.BusService, req.Scheduler, req.Agent, req.AIProvider, req.SystemPrompt)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(respPayload)
	}

	mux.HandleFunc("/skill", handler)
	mux.HandleFunc("/api/skill", handler)

	server := &http.Server{
		Addr:         req.ListenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	fmt.Fprintf(out, "🚀 Kakao i OpenBuilder Skill Server is running on %s\n", req.ListenAddr)
	fmt.Fprintf(out, "  - Skill URL:   http://%s/skill (or /api/skill)\n", req.ListenAddr)
	fmt.Fprintf(out, "  - Health check: http://%s/healthz\n", req.ListenAddr)
	fmt.Fprintf(out, "  - Channel ID:   @%s\n", req.ChannelID)
	if req.Scheduler != nil {
		fmt.Fprintf(out, "  - Cron Scheduler: Active (%d jobs loaded)\n", len(req.Scheduler.ListJobs("")))
	}
	if req.Agent != nil {
		fmt.Fprintf(out, "  - Autonomous Agent: Active (%d tools registered)\n", len(req.Agent.GetTools()))
	} else {
		fmt.Fprintf(out, "  - AI Provider:  %s\n", req.AIProvider.Name())
	}

	errChan := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Fprintln(out, "\nShutting down Skill server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errChan:
		return fmt.Errorf("skill server error: %w", err)
	}
}

func processUtterance(ctx context.Context, utterance, channelID string, busSvc *academy.Service, schedEngine *scheduler.Engine, botAgent agent.Agent, aiProvider ai.Provider, systemPrompt string) *openbuilder.SkillResponse {
	text := strings.ToLower(utterance)

	switch {
	// OpenBuilder verification ping / empty test / greeting
	case text == "" || text == "test" || text == "테스트" || text == "스킬테스트" || text == "스킬 서버 테스트":
		resp := openbuilder.NewSimpleTextResponse("🤖 안녕하세요! 0xc0de1ab AI 챗봇 스킬 서버가 정상 연결되었습니다.\n\n궁금한 점을 메시지로 입력해 주세요!")
		resp.AddQuickReply("도움말", "도움말")
		resp.AddQuickReply("서버 상태", "상태")
		resp.AddQuickReply("정상어학원 버스", "정상어학원 2호차 버스 시간표 알려줘")
		return resp

	case text == "도움말" || text == "help" || text == "?":
		resp := openbuilder.NewSimpleTextResponse(
			"🤖 안녕하세요! 0xc0de1ab AI 챗봇입니다.\n\n" +
				"💡 사용 가능한 질문 예시:\n" +
				"• 🚌 정상어학원 우미린 2차 버스 몇 시에 와?\n" +
				"• ⏰ 매주 평일 15:00에 버스 출발 알림 등록해줘\n" +
				"• 📋 등록된 알림 목록 보여줘\n" +
				"• 💬 서버 상태 확인해줘\n" +
				"• ℹ️ 채널 정보\n\n" +
				"궁금하신 내용을 카카오톡 메시지로 편하게 물어보세요!",
		)
		resp.AddQuickReply("정상어학원 버스", "정상어학원 2호차 버스 시간표 알려줘")
		resp.AddQuickReply("알림 목록", "알림 목록")
		resp.AddQuickReply("서버 상태", "상태")
		return resp

	case text == "알림목록" || text == "알림 목록" || text == "스케줄" || text == "스케줄목록" || text == "예약목록":
		if schedEngine != nil {
			jobs := schedEngine.ListJobs("")
			if len(jobs) == 0 {
				resp := openbuilder.NewSimpleTextResponse("📋 현재 등록된 예약 알림이 없습니다.\n\n예: \"10분 뒤에 라면 끓이기 알림 줘\" 또는 \"매주 평일 15:00에 정상어학원 알림 줘\"")
				resp.AddQuickReply("도움말", "도움말")
				resp.AddQuickReply("정상어학원 버스", "정상어학원 2호차 버스 시간표 알려줘")
				return resp
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("📋 예약된 알림 목록 (총 %d건):\n\n", len(jobs)))
			for i, j := range jobs {
				statusIcon := "🟢"
				if j.Status == scheduler.JobStatusCompleted {
					statusIcon = "✅"
				} else if j.Status == scheduler.JobStatusCancelled {
					statusIcon = "❌"
				}
				sb.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, statusIcon, j.Title))
				if j.Type == scheduler.ScheduleTypeOnce {
					sb.WriteString(fmt.Sprintf("   • 실행: %s\n", j.ExecuteAt.Format("01/02 15:04")))
				} else {
					sb.WriteString(fmt.Sprintf("   • 주기(Cron): %s\n", j.CronExpr))
				}
				sb.WriteString(fmt.Sprintf("   • 내용: %s\n\n", j.Message))
			}

			resp := openbuilder.NewSimpleTextResponse(strings.TrimSpace(sb.String()))
			resp.AddQuickReply("도움말", "도움말")
			resp.AddQuickReply("상태 확인", "상태")
			return resp
		}
		resp := openbuilder.NewSimpleTextResponse("스케줄러가 활성화되지 않았습니다.")
		return resp

	case text == "상태" || text == "status" || text == "서버상태":
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		desc := fmt.Sprintf("OS: %s/%s\nGo: %s\nGoroutines: %d\nAlloc: %.2f MB",
			runtime.GOOS, runtime.GOARCH, runtime.Version(), runtime.NumGoroutine(), float64(m.Alloc)/(1024*1024))

		resp := openbuilder.NewBasicCardResponse(
			"🖥️ 서버 상태 보고서",
			desc,
			"",
			openbuilder.NewMessageButton("다시 조회", "상태"),
			openbuilder.NewMessageButton("도움말", "도움말"),
		)
		return resp

	case text == "시간" || text == "현재시간" || text == "time" || text == "지금몇시" || text == "몇시야" || text == "날짜" || text == "오늘날짜" || text == "요일" || text == "무슨요일":
		now := time.Now().In(holidays.GetKSTLocation())
		info := holidays.CheckDate(now)
		resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf("⏰ 현재 일시: %s (%s) %s\n📅 %s",
			info.Date, info.Weekday, now.Format("15:04:05 KST"), info.Description))
		resp.AddQuickReply("오늘 휴일이야?", "오늘 휴일이야?")
		resp.AddQuickReply("상태 확인", "상태")
		resp.AddQuickReply("도움말", "도움말")
		return resp

	case text == "핑" || text == "ping":
		resp := openbuilder.NewSimpleTextResponse("🏓 퐁! (Pong) 서버가 정상적으로 응답하고 있습니다.")
		return resp

	case strings.Contains(text, "채널") || strings.Contains(text, "정보") || strings.Contains(text, channelID):
		resp := openbuilder.NewBasicCardResponse(
			"🔬 0xc0de1ab Kakao AI Chatbot",
			"카카오톡 채널 @0xc0de1ab AI 연동 공식 챗봇 스킬 서버입니다.",
			"",
			openbuilder.NewWebButton("개발 블로그 / Outline", "https://outline.0xc0de1ab.dev"),
			openbuilder.NewMessageButton("명령어 보기", "도움말"),
		)
		return resp

	default:
		// 1. Fast Path for Holidays (Sub-5ms response)
		if strings.Contains(text, "휴일") || strings.Contains(text, "공휴일") || strings.Contains(text, "쉬는날") || strings.Contains(text, "빨간날") {
			if resp := handleHolidayFastPath(text); resp != nil {
				return resp
			}
		}

		// 2. Fast Path for Scheduler (Sub-10ms response for direct reminder/recurring commands)
		if schedEngine != nil && (strings.Contains(text, "알림") || strings.Contains(text, "예약") || strings.Contains(text, "취소")) {
			if resp := handleScheduleFastPath(schedEngine, text); resp != nil {
				return resp
			}
		}

		// 3. Native TextCard Fast Path for Bus Schedule (Sub-100ms response & native UI)
		if busSvc != nil && isBusQuery(text) && !strings.Contains(text, "알림") && !strings.Contains(text, "예약") {
			if resp := handleBusScheduleFastPath(busSvc, text); resp != nil {
				return resp
			}
		}

		// 2. Autonomous Agent or AI Provider fallback with 4.5s timeout
		aiCtx, cancel := context.WithTimeout(ctx, 4500*time.Millisecond)
		defer cancel()

		if botAgent != nil {
			agentRes, err := botAgent.Run(aiCtx, utterance)
			if err != nil {
				fmt.Fprintf(os.Stderr, "⚠️ [Skill] Agent error for utterance %q: %v\n", utterance, err)
			} else if agentRes != nil && strings.TrimSpace(agentRes.Output) != "" {
				cleanText := stripMarkdown(strings.TrimSpace(agentRes.Output))
				resp := openbuilder.NewSimpleTextResponse(cleanText)
				resp.AddQuickReply("도움말", "도움말")
				resp.AddQuickReply("알림 목록", "알림 목록")
				resp.AddQuickReply("정상어학원 버스", "정상어학원 2호차 버스 시간표 알려줘")
				return resp
			}
		}

		if aiProvider != nil {
			aiResp, err := aiProvider.GenerateResponse(aiCtx, ai.CompletionRequest{
				SystemPrompt: systemPrompt,
				Messages: []ai.ChatMessage{
					{Role: "user", Content: utterance},
				},
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "⚠️ [Skill] AI fallback error for utterance %q: %v\n", utterance, err)
			} else if aiResp != nil && strings.TrimSpace(aiResp.Text) != "" {
				cleanText := stripMarkdown(strings.TrimSpace(aiResp.Text))
				resp := openbuilder.NewSimpleTextResponse(cleanText)
				resp.AddQuickReply("도움말", "도움말")
				resp.AddQuickReply("서버 상태", "상태")
				return resp
			}
		}

		resp := openbuilder.NewSimpleTextResponse("답변을 생성하지 못했습니다. 다시 시도해 주세요.")
		resp.AddQuickReply("도움말", "도움말")
		return resp
	}
}

func isBusQuery(text string) bool {
	keywords := []string{"버스", "시간표", "정류장", "등원", "하원", "기사", "셔틀", "차량", "탑승", "우미린", "양포", "해마루", "현진", "이편한", "중흥", "호반", "원당", "정상어학원", "정상"}
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func handleBusScheduleFastPath(busSvc *academy.Service, text string) *openbuilder.SkillResponse {
	type aliasEntry struct {
		keyword   string
		canonical string
		label     string
	}
	aliases := []aliasEntry{
		{keyword: "우미린더스카이", canonical: "우미린2차", label: "우미린 2차 (더스카이)"},
		{keyword: "더스카이", canonical: "우미린2차", label: "우미린 2차 (더스카이)"},
		{keyword: "센트럴파크", canonical: "우미린2차", label: "우미린 2차 (센트럴파크)"},
		{keyword: "풀하우스", canonical: "우미린1차", label: "우미린 1차 (풀하우스)"},
		{keyword: "우미린 2차", canonical: "우미린2차", label: "우미린 2차"},
		{keyword: "우미린2차", canonical: "우미린2차", label: "우미린 2차"},
		{keyword: "우미린 1차", canonical: "우미린1차", label: "우미린 1차"},
		{keyword: "우미린1차", canonical: "우미린1차", label: "우미린 1차"},
		{keyword: "우미린", canonical: "우미린2차", label: "우미린 2차"},
		{keyword: "해마루초", canonical: "해마루초", label: "해마루초"},
		{keyword: "해마루", canonical: "해마루초", label: "해마루초"},
		{keyword: "이편한", canonical: "이편한", label: "이편한"},
		{keyword: "중흥 1차", canonical: "중흥1차", label: "중흥 1차"},
		{keyword: "중흥1차", canonical: "중흥1차", label: "중흥 1차"},
		{keyword: "중흥", canonical: "중흥", label: "중흥"},
		{keyword: "현진 103동", canonical: "현진 103동", label: "현진 103동"},
		{keyword: "현진 108동", canonical: "현진 108동", label: "현진 108동"},
		{keyword: "현진 남문", canonical: "현진 남문", label: "현진 남문"},
		{keyword: "현진", canonical: "현진", label: "현진"},
		{keyword: "양포도서관", canonical: "양포도서관", label: "양포도서관"},
		{keyword: "양포", canonical: "양포도서관", label: "양포도서관"},
		{keyword: "호반베르디움", canonical: "호반", label: "호반"},
		{keyword: "호반", canonical: "호반", label: "호반"},
		{keyword: "원당초", canonical: "원당초", label: "원당초"},
		{keyword: "원당", canonical: "원당초", label: "원당초"},
	}

	var matchedLoc string
	var displayLocName string
	for _, a := range aliases {
		if strings.Contains(text, a.keyword) {
			matchedLoc = a.canonical
			displayLocName = a.label
			break
		}
	}

	matches := busSvc.Search(academy.SearchQuery{
		Academy:  "정상",
		Location: matchedLoc,
	})

	if len(matches) == 0 {
		return nil
	}

	first := matches[0]

	var sb strings.Builder
	if displayLocName != "" {
		sb.WriteString(fmt.Sprintf("[%s • %s 시간표]\n\n", displayLocName, first.ScheduleType))
	}

	for _, m := range matches {
		locHeader := fmt.Sprintf("📍 %s", m.Location)
		if m.Highlighted {
			locHeader += " ⭐"
		}
		sb.WriteString(locHeader + "\n")

		// Sort class times for consistent presentation
		var classKeys []string
		for cls := range m.Times {
			classKeys = append(classKeys, cls)
		}
		sort.Strings(classKeys)

		for _, cls := range classKeys {
			tm := m.Times[cls]
			if m.Highlighted && strings.Contains(cls, "3시 40분") {
				sb.WriteString(fmt.Sprintf("  👉 %s: %s 📌 (추천 탑승)\n", cls, tm))
			} else {
				sb.WriteString(fmt.Sprintf("  • %s: %s\n", cls, tm))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("💡 3분 전까지 승강장에 대기해 주세요.\n")
	sb.WriteString(fmt.Sprintf("📞 차량 문의: %s", first.Contact))

	phoneClean := strings.ReplaceAll(first.Contact, "-", "")
	phoneClean = strings.ReplaceAll(phoneClean, " ", "")

	cardTitle := fmt.Sprintf("🚌 %s %s", first.AcademyName, first.VehicleNumber)

	resp := openbuilder.NewTextCardResponse(
		cardTitle,
		strings.TrimSpace(sb.String()),
		openbuilder.NewPhoneButton("기사님 전화 연결", phoneClean),
		openbuilder.NewMessageButton("다른 정류장 조회", "정상어학원 버스 시간표 알려줘"),
	)
	resp.AddQuickReply("우미린 2차 시간", "우미린 2차 버스 몇 시에 와?")
	resp.AddQuickReply("양포도서관 시간", "양포도서관 버스 몇 시에 와?")
	resp.AddQuickReply("도움말", "도움말")
	return resp
}

func stripMarkdown(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	s = strings.ReplaceAll(s, "### ", "")
	s = strings.ReplaceAll(s, "## ", "")
	s = strings.ReplaceAll(s, "# ", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "> ", "💡 ")
	return s
}

func handleScheduleFastPath(schedEngine *scheduler.Engine, utterance string) *openbuilder.SkillResponse {
	text := strings.TrimSpace(utterance)
	loc := schedEngine.Location()
	if loc == nil {
		loc = time.Local
	}
	now := time.Now().In(loc)

	// Cancellation: e.g. "알림 취소 job_123" or "취소 job_123"
	if strings.Contains(text, "취소") || strings.Contains(text, "삭제") {
		fields := strings.Fields(text)
		for _, f := range fields {
			if strings.HasPrefix(f, "job_") {
				if err := schedEngine.CancelJob(f); err == nil {
					resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf("✅ 알림 ID %s 가 성공적으로 취소되었습니다.", f))
					resp.AddQuickReply("알림 목록", "알림 목록")
					resp.AddQuickReply("도움말", "도움말")
					return resp
				}
			}
		}
	}

	// Recurring pattern: e.g. "매주 평일 오후 3시에 정상어학원 2호차 등원 알림 등록해줘"
	if strings.Contains(text, "매주") || strings.Contains(text, "매일") || strings.Contains(text, "평일") {
		var cronExpr string
		var hour, min int = 15, 0 // default 15:00

		if strings.Contains(text, "오후 3시") || strings.Contains(text, "15시") || strings.Contains(text, "15:00") {
			hour = 15
		} else if strings.Contains(text, "오전 8시") || strings.Contains(text, "8시") || strings.Contains(text, "08:00") {
			hour = 8
		} else if strings.Contains(text, "오후 4시") || strings.Contains(text, "16시") || strings.Contains(text, "16:00") {
			hour = 16
		} else if strings.Contains(text, "오후 5시") || strings.Contains(text, "17시") || strings.Contains(text, "17:00") {
			hour = 17
		}

		if strings.Contains(text, "평일") || strings.Contains(text, "월~금") || strings.Contains(text, "월요일부터 금요일") {
			cronExpr = fmt.Sprintf("%d %d * * 1-5", min, hour)
		} else if strings.Contains(text, "매일") {
			cronExpr = fmt.Sprintf("%d %d * * *", min, hour)
		} else if strings.Contains(text, "주말") {
			cronExpr = fmt.Sprintf("%d %d * * 6,0", min, hour)
		} else {
			cronExpr = fmt.Sprintf("%d %d * * 1-5", min, hour)
		}

		title := "정상어학원 2호차 등원 알림"
		if strings.Contains(text, "라면") {
			title = "라면 알림"
		} else if strings.Contains(text, "비타민") {
			title = "비타민 복용 알림"
		}

		msg := fmt.Sprintf("%s 시간입니다. (오후 %d:%02d) ⏰", title, hour, min)

		job, err := schedEngine.ScheduleRecurring("kakao_user", title, msg, cronExpr, nil)
		if err == nil {
			resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf(
				"✅ 반복 알림이 등록되었습니다.\n\n• 제목: %s\n• 주기(Cron): %s (매주 평일 %02d:%02d)\n• 내용: %s",
				job.Title, job.CronExpr, hour, min, job.Message,
			))
			resp.AddQuickReply("알림 목록", "알림 목록")
			resp.AddQuickReply("도움말", "도움말")
			return resp
		}
	}

	// Relative one-shot: e.g. "10분 뒤에 라면 물 끄기 알림 등록해줘"
	if strings.Contains(text, "분 뒤") || strings.Contains(text, "분후") || strings.Contains(text, "시간 뒤") || strings.Contains(text, "초 뒤") {
		var dur time.Duration
		fields := strings.Fields(text)
		for _, f := range fields {
			if strings.Contains(f, "분") {
				numStr := strings.TrimRight(f, "분뒤후에 ")
				if n, err := strconv.Atoi(numStr); err == nil {
					dur += time.Duration(n) * time.Minute
				}
			} else if strings.Contains(f, "시간") {
				numStr := strings.TrimRight(f, "시간뒤후에 ")
				if n, err := strconv.Atoi(numStr); err == nil {
					dur += time.Duration(n) * time.Hour
				}
			} else if strings.Contains(f, "초") {
				numStr := strings.TrimRight(f, "초뒤후에 ")
				if n, err := strconv.Atoi(numStr); err == nil {
					dur += time.Duration(n) * time.Second
				}
			}
		}

		if dur > 0 {
			execTime := now.Add(dur)
			title := "예약 알림"
			if strings.Contains(text, "라면") {
				title = "라면 물/불 끄기 알림"
			} else if strings.Contains(text, "버스") {
				title = "학원 버스 알림"
			} else if strings.Contains(text, "세탁") {
				title = "세탁기 확인 알림"
			}

			msg := fmt.Sprintf("약속된 시간이 되었습니다! (%s) ⏰", title)
			job, err := schedEngine.ScheduleOnce("kakao_user", title, msg, execTime, nil)
			if err == nil {
				resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf(
					"✅ 알림이 성공적으로 예약되었습니다.\n\n• 제목: %s\n• 예정 시각: %s (KST)\n• 내용: %s",
					job.Title, job.ExecuteAt.Format("15:04:05"), job.Message,
				))
				resp.AddQuickReply("알림 목록", "알림 목록")
				resp.AddQuickReply("도움말", "도움말")
				return resp
			}
		}
	}

	return nil
}

func handleHolidayFastPath(text string) *openbuilder.SkillResponse {
	// If user asks about upcoming holidays: e.g. "다음 공휴일", "앞으로 공휴일"
	if strings.Contains(text, "다음") || strings.Contains(text, "앞으로") || strings.Contains(text, "다가오는") {
		upcoming := holidays.GetUpcomingHolidays(time.Now(), 5)
		var sb strings.Builder
		sb.WriteString("🗓️ 다가오는 대한민국 공휴일 안내:\n\n")
		for i, u := range upcoming {
			sb.WriteString(fmt.Sprintf("%d. %s (%s) - %s\n", i+1, u.Date, u.Weekday, u.HolidayName))
		}
		resp := openbuilder.NewSimpleTextResponse(strings.TrimSpace(sb.String()))
		resp.AddQuickReply("오늘 휴일이야?", "오늘 휴일이야?")
		resp.AddQuickReply("현재 시간", "시간")
		resp.AddQuickReply("도움말", "도움말")
		return resp
	}

	info, err := holidays.ParseAndCheck(text)
	if err != nil {
		return nil
	}

	var msg string
	if info.IsHoliday {
		subText := ""
		if info.IsSubstituteHoliday {
			subText = " (대체공휴일)"
		}
		msg = fmt.Sprintf("🎉 %s (%s)은 공휴일[%s%s]입니다!", info.Date, info.Weekday, info.HolidayName, subText)
	} else if info.IsWeekend {
		msg = fmt.Sprintf("🏖️ %s (%s)은 주말입니다. (휴일)", info.Date, info.Weekday)
	} else {
		msg = fmt.Sprintf("💼 %s (%s)은 정상 평일(영업일)입니다.", info.Date, info.Weekday)
	}

	resp := openbuilder.NewSimpleTextResponse(msg)
	resp.AddQuickReply("다음 공휴일", "다음 공휴일 언제야?")
	resp.AddQuickReply("현재 시간", "시간")
	resp.AddQuickReply("도움말", "도움말")
	return resp
}

