package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/agent"
	"github.com/dh-kam/kakaotalk-dobby/pkg/openbuilder"
)

// Kakao asks the callback answer to reach the callbackUrl within one minute;
// keep a margin for the delivery round trip itself.
const defaultCallbackProcessingBudget = 50 * time.Second

// DefaultCallbackWaitingMessage is the immediate ack shown while the real
// answer is still being generated.
const DefaultCallbackWaitingMessage = "🧦 도비가 열심히 생각하고 있어요! 조금만 기다려주세요…"

// callbackConfig controls the Kakao Callback API flow: requests that carry a
// one-time callbackUrl are acknowledged immediately and answered later.
type callbackConfig struct {
	Enabled        bool
	WaitingMessage string
	ProcessingTime time.Duration
	HTTPClient     *http.Client
}

func newCallbackConfig(enabled bool, waitingMessage string, processingTime time.Duration) callbackConfig {
	if waitingMessage == "" {
		waitingMessage = DefaultCallbackWaitingMessage
	}
	if processingTime <= 0 {
		processingTime = defaultCallbackProcessingBudget
	}
	return callbackConfig{
		Enabled:        enabled,
		WaitingMessage: waitingMessage,
		ProcessingTime: processingTime,
		HTTPClient:     &http.Client{Timeout: 10 * time.Second},
	}
}

var allowedCallbackDomains = []string{
	"kakao.com",
	"kakaowork.com",
	"daum.net",
	"0xc0de1ab.dev",
}

func isAllowedDomain(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, domain := range allowedCallbackDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

// callbackEligible reports whether the URL may be used for callback delivery.
// It strictly validates that external targets are HTTPS and belong to legitimate
// Kakao platform or authorized domains, preventing Server-Side Request Forgery (SSRF).
func callbackEligible(rawURL string) bool {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return false
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Host == "" {
		return false
	}

	host := strings.ToLower(u.Hostname())

	// Loopback is allowed for local testing (HTTP or HTTPS)
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return u.Scheme == "http" || u.Scheme == "https"
	}

	// External targets MUST use HTTPS and belong to verified platform domains
	if u.Scheme != "https" {
		return false
	}

	return isAllowedDomain(host)
}

// kakaoCallbackAck is the CallbackResponse Kakao returns after a delivery.
type kakaoCallbackAck struct {
	TaskID    string `json:"taskId"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// deliverCallbackResponse POSTs the final response to the one-time callback
// URL and verifies Kakao accepted it.
func deliverCallbackResponse(ctx context.Context, client *http.Client, callbackURL string, resp *openbuilder.SkillResponse, out io.Writer) error {
	body, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal callback response: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build callback request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")

	httpResp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("post callback response: %w", err)
	}
	defer httpResp.Body.Close()

	var ack kakaoCallbackAck
	if err := json.NewDecoder(io.LimitReader(httpResp.Body, 16*1024)).Decode(&ack); err != nil {
		return fmt.Errorf("decode callback ack (HTTP %d): %w", httpResp.StatusCode, err)
	}
	if ack.Status != "SUCCESS" {
		return fmt.Errorf("callback rejected (status %s): %s", ack.Status, ack.Message)
	}

	fmt.Fprintf(out, "[Callback] Delivered answer for task %s\n", ack.TaskID)
	return nil
}

// runCallbackTurn processes the turn under the extended callback budget and
// delivers the result to the callback URL. It intentionally does not hold the
// per-user lock: a slow thinking turn must not block the user's next message
// (the session store is synchronized internally).
func runCallbackTurn(ctx context.Context, cb callbackConfig, callbackURL, utterance, userID string, req SkillServeRequest, sessionStore agent.SessionStore, slashRegistry *SlashCommandRegistry, out io.Writer) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "⚠️ [Callback] recovered from panic for user %s: %v\n", userID, r)
		}
	}()

	// Base on the server context, not r.Context(): the HTTP request ends as
	// soon as the ack is written, while shutdown should still abort delivery.
	bgCtx, cancel := context.WithTimeout(ctx, cb.ProcessingTime)
	defer cancel()

	resp := processUtterance(bgCtx, utterance, userID, req.ChannelID, req.DataDir, req.AdminToken, req.BusService, req.SchoolService, req.Scheduler, req.Agent, req.AIProvider, sessionStore, req.SystemPrompt, slashRegistry)
	if resp == nil {
		resp = openbuilder.NewSimpleTextResponse("죄송해요, 답변을 준비하지 못했어요. 잠시 후 다시 물어봐 주세요!")
	}
	resp.ValidateAndNormalize()

	if err := deliverCallbackResponse(bgCtx, cb.HTTPClient, callbackURL, resp, out); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ [Callback] delivery failed for user %s: %v\n", userID, err)
	}
}
