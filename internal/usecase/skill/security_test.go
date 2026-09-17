package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/agent"
	"github.com/dh-kam/kakaotalk-dobby/pkg/ai"
	"github.com/dh-kam/kakaotalk-dobby/pkg/openbuilder"
	"github.com/dh-kam/kakaotalk-dobby/pkg/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startSecurityTestServer(t *testing.T, req SkillServeRequest) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	req.ListenAddr = listener.Addr().String()
	require.NoError(t, listener.Close())
	if req.Out == nil {
		req.Out = io.Discard
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- NewSkillServeUseCase().Execute(ctx, req) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Error("skill server did not shut down")
		}
	})
	baseURL := "http://" + req.ListenAddr
	waitForServer(t, baseURL+"/healthz")
	return baseURL
}

func TestSkillHTTPAuthenticationAndIdentity(t *testing.T) {
	engine := scheduler.NewEngine(scheduler.NewMemoryStore(), nil)
	job, err := engine.ScheduleOnce("alice", "private-alice-title", "private message", time.Now().Add(time.Hour), nil)
	require.NoError(t, err)
	sessions := agent.NewMemorySessionStore(10)
	sessions.AppendTurn("alice", "private conversation", "private answer")
	baseURL := startSecurityTestServer(t, SkillServeRequest{
		SkillToken: testSkillToken, AdminToken: "separate-admin-token",
		Scheduler: engine, SessionStore: sessions, AIProvider: ai.NewMockProvider("mock"),
	})

	cases := []struct {
		name, user, utterance, header, token string
		status                               int
	}{
		{"wildcard without authentication", "*", "/alarm list", "", "", http.StatusUnauthorized},
		{"forged cancellation without authentication", "alice", "/alarm cancel " + job.ID, "", "", http.StatusUnauthorized},
		{"forged reset without authentication", "alice", "/reset", "", "", http.StatusUnauthorized},
		{"wrong token", "alice", "/alarm list", "X-Skill-Token", "wrong", http.StatusUnauthorized},
		{"admin token is not skill auth", "alice", "/alarm list", "X-Admin-Token", "separate-admin-token", http.StatusUnauthorized},
		{"wildcard with authentication", "*", "/alarm list", "X-Skill-Token", testSkillToken, http.StatusBadRequest},
		{"whitespace ID", "   ", "알림 목록", "X-Skill-Token", testSkillToken, http.StatusBadRequest},
		{"missing ID", "", "알림 목록", "X-Skill-Token", testSkillToken, http.StatusBadRequest},
		{"anonymous ID", "anonymous", "알림 목록", "X-Skill-Token", testSkillToken, http.StatusBadRequest},
		{"padded ID", " alice ", "알림 목록", "X-Skill-Token", testSkillToken, http.StatusBadRequest},
		{"authenticated other user", "bob", "/alarm list", "X-Skill-Token", testSkillToken, http.StatusOK},
		{"case-sensitive user", "Alice", "/alarm list", "X-Skill-Token", testSkillToken, http.StatusOK},
	}
	for _, path := range []string{"/skill", "/api/skill"} {
		for _, tc := range cases {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				payload := openbuilder.SkillPayload{UserRequest: openbuilder.UserRequest{
					User: openbuilder.ChatUser{ID: tc.user}, Utterance: tc.utterance,
				}}
				body, err := json.Marshal(payload)
				require.NoError(t, err)
				req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(body))
				require.NoError(t, err)
				if tc.header != "" {
					req.Header.Set(tc.header, tc.token)
				}
				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()
				got, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				assert.Equal(t, tc.status, resp.StatusCode)
				assert.NotContains(t, string(got), "private-alice-title")
				assert.NotContains(t, string(got), "private conversation")
			})
		}
	}
	after, err := engine.GetJob(job.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduler.JobStatusActive, after.Status)
	require.Len(t, sessions.GetHistory("alice"), 2)

	// The trusted OpenBuilder sender still has access to the exact owner's jobs.
	payload := `{"userRequest":{"user":{"id":"alice"},"utterance":"/alarm list"}}`
	req, err := http.NewRequest(http.MethodPost, baseURL+"/skill", strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+testSkillToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "private-alice-title")
}

type historyCaptureProvider struct {
	mu       sync.Mutex
	messages []ai.ChatMessage
}

func (p *historyCaptureProvider) Name() string  { return "capture" }
func (p *historyCaptureProvider) Model() string { return "mock" }
func (p *historyCaptureProvider) GenerateResponse(_ context.Context, req ai.CompletionRequest) (*ai.CompletionResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.messages = req.Messages
	return &ai.CompletionResponse{Text: "captured answer"}, nil
}

func TestAdminCommandsNeverEnterHistoryOrLogs(t *testing.T) {
	sessions := agent.NewMemorySessionStore(50)
	provider := &historyCaptureProvider{}
	var logs bytes.Buffer
	logWriter := &safeWriter{w: &logs}
	baseURL := startSecurityTestServer(t, SkillServeRequest{
		SkillToken: testSkillToken, AdminToken: "admin-secret-marker",
		DataDir: t.TempDir(), SessionStore: sessions, AIProvider: provider, Out: logWriter,
	})
	for _, utterance := range []string{
		"/data list admin-secret-marker", "/DATA reload admin-secret-marker",
		"/catalog list admin-secret-marker", "/데이터 list admin-secret-marker",
		"/data list wrong-secret-marker", "/data unknown admin-secret-marker",
	} {
		resp := sendSkillMessage(t, baseURL, utterance)
		require.NotEmpty(t, resp.Template.Outputs)
	}
	assert.Empty(t, sessions.GetHistory("e2e-test-user-01"))
	logWriter.mu.Lock()
	assert.NotContains(t, logs.String(), "admin-secret-marker")
	assert.NotContains(t, logs.String(), "wrong-secret-marker")
	logWriter.mu.Unlock()
	sendSkillMessage(t, baseURL, "이전 대화를 설명해줘")
	provider.mu.Lock()
	defer provider.mu.Unlock()
	require.Len(t, provider.messages, 1)
	assert.Equal(t, "이전 대화를 설명해줘", provider.messages[0].Content)
}

func TestSkillServeRequiresToken(t *testing.T) {
	for _, token := range []string{"", "  ", " padded "} {
		err := NewSkillServeUseCase().Execute(t.Context(), SkillServeRequest{SkillToken: token, Out: io.Discard})
		require.Error(t, err)
	}
	assert.Error(t, ValidateServeOptions(testSkillToken, -time.Second))
	assert.Error(t, ValidateServeOptions(testSkillToken, 51*time.Second))
	assert.NoError(t, ValidateServeOptions(testSkillToken, 50*time.Second))
}
