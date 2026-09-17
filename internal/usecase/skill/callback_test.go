package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/agent"
	"github.com/dh-kam/kakaotalk-dobby/pkg/ai"
	"github.com/dh-kam/kakaotalk-dobby/pkg/openbuilder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testCallbackURL = "https://bot-api.kakao.com/v2/bot/callback/test-task"

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

// Replace only the transport in tests. The production URL validator and
// no-redirect policy remain enabled, without exposing a local-URL bypass flag.
func localCallbackClient(t *testing.T, receiver *httptest.Server) *http.Client {
	t.Helper()
	endpoint, err := url.Parse(receiver.URL)
	require.NoError(t, err)
	client := newCallbackConfig(true, "", 0).HTTPClient
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		clone := r.Clone(r.Context())
		clone.URL.Scheme = endpoint.Scheme
		clone.URL.Host = endpoint.Host
		return receiver.Client().Transport.RoundTrip(clone)
	})
	return client
}

type staticCallbackResolver []netip.Addr

func (r staticCallbackResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return r, nil
}

func TestCallbackDialerRejectsNonPublicDNS(t *testing.T) {
	for _, address := range []string{
		"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254",
		"100.100.100.200", "0.0.0.0", "::1", "::ffff:127.0.0.1", "fd00::1", "fe80::1",
		"64:ff9b::a00:1", "198.18.0.1", "224.0.0.1",
	} {
		t.Run(address, func(t *testing.T) {
			// A public answer followed by a private answer must reject the entire
			// DNS result before making any connection.
			dialer := callbackDialer{resolver: staticCallbackResolver{
				netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr(address),
			}}
			conn, err := dialer.DialContext(t.Context(), "tcp", "bot-api.kakao.com:443")
			require.ErrorContains(t, err, "non-public")
			assert.Nil(t, conn)
		})
	}
	assert.True(t, isPublicCallbackIP(netip.MustParseAddr("8.8.8.8")))
	assert.True(t, isPublicCallbackIP(netip.MustParseAddr("2606:4700:4700::1111")))
}

func TestCallbackDeliveryRejectsRedirectsAndErrorStatus(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308, 400, 500} {
		client := newCallbackConfig(true, "", 0).HTTPClient
		calls := 0
		client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{
				StatusCode: status,
				Header:     http.Header{"Location": []string{"http://127.0.0.1/internal"}},
				Body:       io.NopCloser(strings.NewReader(`{"status":"SUCCESS"}`)), Request: r,
			}, nil
		})
		err := deliverCallbackResponse(t.Context(), client, testCallbackURL, openbuilder.NewSimpleTextResponse("answer"), io.Discard)
		require.Error(t, err, "HTTP %d cannot acknowledge a callback", status)
		assert.Equal(t, 1, calls, "HTTP %d must not follow a redirect", status)
	}
}

func TestCallbackDeliveryRejectsLoopbackBeforeTransport(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid callback URL reached transport")
		return nil, nil
	})}
	err := deliverCallbackResponse(t.Context(), client, "http://127.0.0.1:9999/internal", openbuilder.NewSimpleTextResponse("answer"), io.Discard)
	require.ErrorContains(t, err, "invalid callback URL")
}

type callbackBudgetAgent struct {
	blockingAgent
	remaining time.Duration
}

func (a *callbackBudgetAgent) RunWithHistory(ctx context.Context, _ string, _ []agent.Message) (*agent.AgentResult, error) {
	deadline, _ := ctx.Deadline()
	a.remaining = time.Until(deadline)
	select {
	case <-time.After(a.delay):
		return &agent.AgentResult{Output: "slow agent answer"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestCallbackAgentCanFinishAfterThreeSeconds(t *testing.T) {
	var received openbuilder.SkillResponse
	cb := newCallbackConfig(true, "", 50*time.Second)
	cb.HTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		err := json.NewDecoder(r.Body).Decode(&received)
		require.NoError(t, err)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"SUCCESS"}`)), Header: make(http.Header)}, nil
	})
	bot := &callbackBudgetAgent{blockingAgent: blockingAgent{delay: 3100 * time.Millisecond}}
	runCallbackTurn(t.Context(), cb, testCallbackURL, "분석해줘", "user", SkillServeRequest{Agent: bot, ChannelID: "channel"}, nil, nil, io.Discard)
	assert.Greater(t, bot.remaining, 48*time.Second)
	assert.LessOrEqual(t, bot.remaining, 50*time.Second)
	assert.Contains(t, extractResponseText(&received), "slow agent answer")
}

func TestCallbackDeliverySurvivesGenerationDeadline(t *testing.T) {
	registry := NewSlashCommandRegistry()
	registry.Register(CommandSpec{Name: "slow", Handler: func(cc *CommandContext) *openbuilder.SkillResponse {
		<-cc.Ctx.Done()
		return openbuilder.NewSimpleTextResponse("generation timed out")
	}})
	cb := newCallbackConfig(true, "", 10*time.Millisecond)
	delivered := false
	cb.HTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.NoError(t, r.Context().Err())
		deadline, ok := r.Context().Deadline()
		require.True(t, ok)
		assert.Greater(t, time.Until(deadline), 4*time.Second)
		delivered = true
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"SUCCESS"}`)), Header: make(http.Header)}, nil
	})
	runCallbackTurn(t.Context(), cb, testCallbackURL, "/slow", "user", SkillServeRequest{}, nil, registry, io.Discard)
	assert.True(t, delivered)
}

// slowProvider simulates an LLM that cannot answer inside the Kakao 5s SLA.
type slowProvider struct {
	delay time.Duration
}

func (p *slowProvider) Name() string { return "slow" }
func (p *slowProvider) Model() string {
	return "slow-test-model"
}
func (p *slowProvider) GenerateResponse(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResponse, error) {
	select {
	case <-time.After(p.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &ai.CompletionResponse{Text: "느린 모델의 최종 답변입니다"}, nil
}

func TestCallbackEligible(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"empty", "", false},
		{"blank", "   ", false},
		{"https kakao bot-api allowed", "https://bot-api.kakao.com/v2/bot/callback", true},
		{"https kakaowork allowed", "https://api.kakaowork.com/callback", true},
		{"https 0xc0de1ab allowed", "https://webhook.0xc0de1ab.dev/callback", true},
		{"https evil rejected (SSRF defense)", "https://evil.example/callback", false},
		{"https metadata ip rejected (SSRF defense)", "https://169.254.169.254/computeMetadata/v1", false},
		{"https internal k8s rejected (SSRF defense)", "https://kubernetes.default.svc:443", false},
		{"http loopback rejected", "http://127.0.0.1:9999/cb", false},
		{"http localhost rejected", "http://localhost:9999/cb", false},
		{"https loopback rejected", "https://127.0.0.1/cb", false},
		{"IPv6 loopback rejected", "https://[::1]/cb", false},
		{"userinfo rejected", "https://user:password@bot-api.kakao.com/cb", false},
		{"custom port rejected", "https://bot-api.kakao.com:8443/cb", false},
		{"HTTPS port allowed", "https://bot-api.kakao.com:443/cb", true},
		{"domain suffix attack rejected", "https://notkakao.com/cb", false},
		{"domain prefix attack rejected", "https://kakao.com.evil.example/cb", false},
		{"http external rejected", "http://bot-api.kakao.com/callback", false},
		{"ftp rejected", "ftp://bot-api.kakao.com/callback", false},
		{"garbage rejected", "not a url", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, callbackEligible(tt.url), tt.name)
	}
}

func TestNewCallbackResponseShape(t *testing.T) {
	data, err := json.Marshal(openbuilder.NewCallbackResponse("기다려주세요"))
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Equal(t, true, raw["useCallback"])
	assert.Equal(t, "2.0", raw["version"])
	assert.Equal(t, "기다려주세요", raw["data"])
}

func TestSkillHandler_CallbackFlow(t *testing.T) {
	var mu sync.Mutex
	var deliveredBodies []json.RawMessage
	delivered := make(chan struct{}, 4)

	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		deliveredBodies = append(deliveredBodies, json.RawMessage(body))
		mu.Unlock()
		delivered <- struct{}{}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"taskId": "task-uuid-1", "status": "SUCCESS", "timestamp": 1700000000}`))
	}))
	defer receiver.Close()

	serverAddr := "127.0.0.1:18123"
	var serverOutput bytes.Buffer
	uc := NewSkillServeUseCase()
	uc.callbackHTTPClient = localCallbackClient(t, receiver)
	ctx, cancel := contextWithTimeout(t, 15*time.Second)
	defer cancel()
	go func() {
		_ = uc.Execute(ctx, SkillServeRequest{
			SkillToken:             testSkillToken,
			ListenAddr:             serverAddr,
			ChannelID:              "0xc0de1ab",
			AIProvider:             &slowProvider{delay: 300 * time.Millisecond},
			SessionStore:           agent.NewMemorySessionStore(10),
			CallbackEnabled:        true,
			CallbackWaitingMessage: "조금만 기다려주세요",
			Out:                    &serverOutput,
		})
	}()

	baseURL := "http://" + serverAddr
	waitForServer(t, baseURL+"/healthz")

	payload := map[string]interface{}{
		"userRequest": map[string]interface{}{
			"timezone":    "Asia/Seoul",
			"utterance":   "천천히 대답해줘",
			"lang":        "kr",
			"callbackUrl": testCallbackURL,
			"user":        map[string]interface{}{"id": "cb-user", "type": "accountId"},
		},
	}
	body, _ := json.Marshal(payload)

	start := time.Now()
	httpResp, err := postSkillRequest(t, baseURL+"/skill", bytes.NewReader(body))
	require.NoError(t, err)
	defer httpResp.Body.Close()

	var ack openbuilder.SkillResponse
	require.NoError(t, json.NewDecoder(httpResp.Body).Decode(&ack))
	elapsed := time.Since(start)

	// The ack must return long before the 300ms slow model could answer.
	assert.True(t, ack.UseCallback, "immediate reply must be the callback ack")
	assert.Equal(t, "조금만 기다려주세요", ack.Data)
	assert.Less(t, elapsed, 250*time.Millisecond, "ack must not wait for the slow model")

	select {
	case <-delivered:
	case <-time.After(5 * time.Second):
		t.Fatal("callback answer was never delivered")
	}

	mu.Lock()
	require.Len(t, deliveredBodies, 1)
	var final openbuilder.SkillResponse
	require.NoError(t, json.Unmarshal(deliveredBodies[0], &final))
	mu.Unlock()

	assert.False(t, final.UseCallback, "delivered answer is a plain skill response")
	require.NotEmpty(t, final.Template.Outputs)
	assert.Equal(t, "느린 모델의 최종 답변입니다", final.Template.Outputs[0].SimpleText.Text)
}

func TestSkillHandler_SyncFlowWithoutCallbackURL(t *testing.T) {
	serverAddr := "127.0.0.1:18124"
	var serverOutput bytes.Buffer
	uc := NewSkillServeUseCase()
	ctx, cancel := contextWithTimeout(t, 15*time.Second)
	defer cancel()
	go func() {
		_ = uc.Execute(ctx, SkillServeRequest{
			SkillToken:      testSkillToken,
			ListenAddr:      serverAddr,
			ChannelID:       "0xc0de1ab",
			AIProvider:      ai.NewMockProvider("mock"),
			SessionStore:    agent.NewMemorySessionStore(10),
			CallbackEnabled: true,
			Out:             &serverOutput,
		})
	}()

	baseURL := "http://" + serverAddr
	waitForServer(t, baseURL+"/healthz")

	resp := sendSkillMessage(t, baseURL, "그냥 동기 응답해줘")
	assert.False(t, resp.UseCallback)
	assert.NotEmpty(t, resp.Template.Outputs)
}

func TestSkillHandler_InsecureCallbackURLFallsBackToSync(t *testing.T) {
	serverAddr := "127.0.0.1:18125"
	var serverOutput bytes.Buffer
	uc := NewSkillServeUseCase()
	ctx, cancel := contextWithTimeout(t, 15*time.Second)
	defer cancel()
	go func() {
		_ = uc.Execute(ctx, SkillServeRequest{
			SkillToken:             testSkillToken,
			ListenAddr:             serverAddr,
			ChannelID:              "0xc0de1ab",
			AIProvider:             ai.NewMockProvider("mock"),
			SessionStore:           agent.NewMemorySessionStore(10),
			CallbackEnabled:        true,
			CallbackWaitingMessage: "기다려",
			Out:                    &serverOutput,
		})
	}()

	baseURL := "http://" + serverAddr
	waitForServer(t, baseURL+"/healthz")

	payload := map[string]interface{}{
		"userRequest": map[string]interface{}{
			"utterance":   "동기로 흘러야 해",
			"callbackUrl": "http://external.example/callback",
			"user":        map[string]interface{}{"id": "evil-cb", "type": "accountId"},
		},
	}
	body, _ := json.Marshal(payload)
	httpResp, err := postSkillRequest(t, baseURL+"/skill", bytes.NewReader(body))
	require.NoError(t, err)
	defer httpResp.Body.Close()

	var resp openbuilder.SkillResponse
	require.NoError(t, json.NewDecoder(httpResp.Body).Decode(&resp))
	assert.False(t, resp.UseCallback, "non-HTTPS external callbackUrl must not enter the callback flow")
	assert.NotEmpty(t, resp.Template.Outputs)
}
