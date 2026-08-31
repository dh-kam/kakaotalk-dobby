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

		respPayload := processUtterance(r.Context(), utterance, req.ChannelID, req.Agent, req.AIProvider, req.SystemPrompt)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(respPayload)
	}

	mux.HandleFunc("/skill", handler)
	mux.HandleFunc("/api/skill", handler)

	server := &http.Server{
		Addr:         req.ListenAddr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
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

func processUtterance(ctx context.Context, utterance, channelID string, botAgent agent.Agent, aiProvider ai.Provider, systemPrompt string) *openbuilder.SkillResponse {
	text := strings.ToLower(utterance)

	switch {
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
		// Forward to Autonomous Agent or AI Provider with 3.5s timeout to guarantee safety under Kakao 5s limit
		aiCtx, cancel := context.WithTimeout(ctx, 3500*time.Millisecond)
		defer cancel()

		if botAgent != nil {
			agentRes, err := botAgent.Run(aiCtx, utterance)
			if err == nil && agentRes != nil && strings.TrimSpace(agentRes.Output) != "" {
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
			if err == nil && aiResp != nil && strings.TrimSpace(aiResp.Text) != "" {
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
