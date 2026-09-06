package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/agent"
	"github.com/dh-kam/kakaotalk-dobby/pkg/ai"
	"github.com/dh-kam/kakaotalk-dobby/pkg/openbuilder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		{"http loopback allowed", "http://127.0.0.1:9999/cb", true},
		{"http localhost allowed", "http://localhost:9999/cb", true},
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
	ctx, cancel := contextWithTimeout(t, 15*time.Second)
	defer cancel()
	go func() {
		_ = uc.Execute(ctx, SkillServeRequest{
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
			"callbackUrl": receiver.URL,
			"user":        map[string]interface{}{"id": "cb-user", "type": "accountId"},
		},
	}
	body, _ := json.Marshal(payload)

	start := time.Now()
	httpResp, err := http.Post(baseURL+"/skill", "application/json", bytes.NewReader(body))
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
	httpResp, err := http.Post(baseURL+"/skill", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer httpResp.Body.Close()

	var resp openbuilder.SkillResponse
	require.NoError(t, json.NewDecoder(httpResp.Body).Decode(&resp))
	assert.False(t, resp.UseCallback, "non-HTTPS external callbackUrl must not enter the callback flow")
	assert.NotEmpty(t, resp.Template.Outputs)
}
