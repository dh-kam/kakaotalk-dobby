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

	"github.com/dh-kam/kakao-bot/pkg/ai"
	"github.com/dh-kam/kakao-bot/pkg/openbuilder"
)

// SkillServeRequest holds options for the OpenBuilder Skill Server.
type SkillServeRequest struct {
	ListenAddr   string
	ChannelID    string
	AIProvider   ai.Provider
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

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var payload openbuilder.SkillPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "Invalid OpenBuilder JSON", http.StatusBadRequest)
			return
		}

		utterance := strings.TrimSpace(payload.UserRequest.Utterance)
		fmt.Fprintf(out, "[Skill Request] User: %s | Message: %q\n", payload.UserRequest.User.ID, utterance)

		response := processUtterance(r.Context(), utterance, req.ChannelID, req.AIProvider, req.SystemPrompt)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
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
	fmt.Fprintf(out, "  - AI Provider:  %s\n", req.AIProvider.Name())

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

func processUtterance(ctx context.Context, utterance, channelID string, aiProvider ai.Provider, systemPrompt string) *openbuilder.SkillResponse {
	text := strings.ToLower(utterance)

	switch {
	case text == "도움말" || text == "help" || text == "?":
		resp := openbuilder.NewSimpleTextResponse(
			"🤖 안녕하세요! 0xc0de1ab AI 챗봇입니다.\n\n사용 가능한 명령어:\n- 💬 상태: 서버 상태 및 가동 시간 확인\n- ⏰ 시간: 서버 현재 시각 확인\n- 🏓 핑: 응답 테스트 (Ping-Pong)\n- ℹ️ 채널: 채널 정보 및 링크\n\n💡 원하는 질문을 자유롭게 입력하시면 AI가 직접 답변해 드립니다!",
		)
		resp.AddQuickReply("서버 상태", "상태")
		resp.AddQuickReply("현재 시간", "시간")
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
			openbuilder.NewMessageButton("새로고침", "상태"),
			openbuilder.NewMessageButton("도움말", "도움말"),
		)
		return resp

	case text == "시간" || text == "time" || text == "현재시간":
		now := time.Now().Format("2006-01-02 15:04:05 (MST)")
		resp := openbuilder.NewSimpleTextResponse(fmt.Sprintf("⏰ 현재 서버 시간:\n%s", now))
		resp.AddQuickReply("서버 상태", "상태")
		resp.AddQuickReply("도움말", "도움말")
		return resp

	case text == "핑" || text == "ping":
		resp := openbuilder.NewSimpleTextResponse("🏓 퐁! 정상 동작 중입니다.")
		resp.AddQuickReply("서버 상태", "상태")
		return resp

	case strings.Contains(text, "채널") || strings.Contains(text, "정보") || strings.Contains(text, channelID):
		resp := openbuilder.NewBasicCardResponse(
			fmt.Sprintf("🔬 0xc0de1ab Kakao AI Chatbot"),
			"카카오톡 채널 @0xc0de1ab AI 연동 공식 챗봇 스킬 서버입니다.",
			"",
			openbuilder.NewWebButton("개발 블로그 / Outline", "https://outline.0xc0de1ab.dev"),
			openbuilder.NewMessageButton("명령어 보기", "도움말"),
		)
		return resp

	default:
		// Forward to AI Provider with 3.5s timeout to guarantee safety under Kakao 5s limit
		aiCtx, cancel := context.WithTimeout(ctx, 3500*time.Millisecond)
		defer cancel()

		aiResp, err := aiProvider.GenerateResponse(aiCtx, ai.CompletionRequest{
			SystemPrompt: systemPrompt,
			Messages: []ai.ChatMessage{
				{Role: "user", Content: utterance},
			},
		})

		if err != nil {
			resp := openbuilder.NewSimpleTextResponse(
				fmt.Sprintf("⚠️ AI 응답 생성 중 오류가 발생했습니다: %v\n\n잠시 후 다시 시도해 주세요.", err),
			)
			resp.AddQuickReply("도움말", "도움말")
			return resp
		}

		replyText := strings.TrimSpace(aiResp.Text)
		if replyText == "" {
			replyText = "답변을 생성하지 못했습니다."
		}

		resp := openbuilder.NewSimpleTextResponse(replyText)
		resp.AddQuickReply("도움말", "도움말")
		resp.AddQuickReply("서버 상태", "상태")
		return resp
	}
}
