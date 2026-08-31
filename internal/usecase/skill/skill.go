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
	"strings"
	"time"

	"github.com/dh-kam/kakao-bot/pkg/academy"
	"github.com/dh-kam/kakao-bot/pkg/agent"
	"github.com/dh-kam/kakao-bot/pkg/ai"
	"github.com/dh-kam/kakao-bot/pkg/openbuilder"
)

// SkillServeRequest holds options for the OpenBuilder Skill Server.
type SkillServeRequest struct {
	ListenAddr   string
	ChannelID    string
	AIProvider   ai.Provider
	Agent        agent.Agent
	BusService   *academy.Service
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

		respPayload := processUtterance(r.Context(), utterance, req.ChannelID, req.BusService, req.Agent, req.AIProvider, req.SystemPrompt)

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

func processUtterance(ctx context.Context, utterance, channelID string, busSvc *academy.Service, botAgent agent.Agent, aiProvider ai.Provider, systemPrompt string) *openbuilder.SkillResponse {
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
			"🤖 안녕하세요! 0xc0de1ab AI 챗봇입니다.\n\n사용 가능한 질문 예시:\n- 🚌 \"정상어학원 우미린 2차 버스 몇 시에 와?\"\n- 🚌 \"정상어학원 2호차 등원 시간표 알려줘\"\n- 💬 \"서버 상태 확인해줘\"\n- ℹ️ \"채널 정보\"\n\n💡 원하는 질문을 자유롭게 입력하시면 AI Agent가 도구를 호출하여 정확히 답변해 드립니다!",
		)
		resp.AddQuickReply("정상어학원 버스", "정상어학원 2호차 버스 시간표 알려줘")
		resp.AddQuickReply("서버 상태", "상태")
		resp.AddQuickReply("채널 정보", "채널")
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

	case text == "시간" || text == "현재시간" || text == "time":
		now := time.Now().Format("2006년 01월 02일 15:04:05 (MST)")
		resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf("⏰ 현재 서버 시각: %s", now))
		resp.AddQuickReply("상태 확인", "상태")
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
		// 1. Fast Path for Bus Schedule (Sub-10ms response to guarantee safety under Kakao 5s timeout)
		if busSvc != nil && isBusQuery(text) {
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
				resp := openbuilder.NewSimpleTextResponse(strings.TrimSpace(agentRes.Output))
				resp.AddQuickReply("도움말", "도움말")
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
				resp := openbuilder.NewSimpleTextResponse(strings.TrimSpace(aiResp.Text))
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
	// Extract stop / location keyword
	locations := []string{
		"우미린2차", "우미린 2차", "우미린1차", "우미린 1차", "우미린",
		"해마루초", "이편한", "중흥1차", "중흥 1차", "중흥",
		"현진 103동", "현진 108동", "현진 남문", "현진",
		"양포도서관", "양포", "호반베르디움", "호반", "원당초", "원당",
	}

	var matchedLoc string
	for _, loc := range locations {
		if strings.Contains(text, strings.ToLower(loc)) {
			matchedLoc = loc
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

	var sb strings.Builder
	first := matches[0]
	sb.WriteString(fmt.Sprintf("🚌 %s (%s %s 시간표)\n\n", first.AcademyName, first.VehicleNumber, first.ScheduleType))

	for i, m := range matches {
		if i >= 4 {
			break
		}
		sb.WriteString(fmt.Sprintf("📍 **%s**\n", m.Location))
		for cls, tm := range m.Times {
			sb.WriteString(fmt.Sprintf("  • %s: **%s**\n", cls, tm))
		}
		if m.Note != "" {
			sb.WriteString(fmt.Sprintf("  📌 %s\n", m.Note))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("💡 **안내 사항**\n")
	sb.WriteString("• 표기된 시간은 출발 시간이니 **3분 전 대기** 바랍니다.\n")
	if first.Contact != "" {
		sb.WriteString(fmt.Sprintf("• 차량 문의: **%s** (%s)\n", first.Contact, first.OperatingHours))
	}

	resp := openbuilder.NewSimpleTextResponse(strings.TrimSpace(sb.String()))
	resp.AddQuickReply("우미린 2차 시간", "우미린 2차 버스 몇 시에 와?")
	resp.AddQuickReply("양포도서관 시간", "양포도서관 버스 몇 시에 와?")
	resp.AddQuickReply("도움말", "도움말")
	return resp
}
