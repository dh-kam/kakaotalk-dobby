package skill

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/agent"
	"github.com/dh-kam/kakaotalk-dobby/pkg/ai"
	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
	"github.com/dh-kam/kakaotalk-dobby/pkg/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduleAPI(t *testing.T) {
	store := scheduler.NewMemoryStore()
	engine := scheduler.NewEngine(store, func(ctx context.Context, job *scheduler.Job) error { return nil })
	err := engine.Start(t.Context())
	require.NoError(t, err)
	defer engine.Stop()

	handler := newScheduleHandler(engine, "test-secret-token")

	t.Run("unauthorized access returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/schedules", nil)
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("create once schedule succeeds", func(t *testing.T) {
		payload := `{
			"title": "치과 예약 알림",
			"message": "오후 3시 치과 방문",
			"execute_at": "1h",
			"bot": "dobby"
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/schedules", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		var resp struct {
			Success bool           `json:"success"`
			Job     *scheduler.Job `json:"job"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "치과 예약 알림", resp.Job.Title)
		assert.Equal(t, "dobby", resp.Job.Payload["bot"])
	})

	t.Run("create recurring schedule succeeds", func(t *testing.T) {
		payload := `{
			"title": "매일 아침 버스 알림",
			"message": "8시 10분 등교 버스",
			"type": "recurring",
			"cron_expr": "10 8 * * 1-5"
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/schedules", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("list schedules returns active jobs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/schedules", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Jobs  []*scheduler.Job `json:"jobs"`
			Count int              `json:"count"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Count)
	})

	t.Run("delete schedule succeeds", func(t *testing.T) {
		jobs := engine.ListAllJobs()
		require.NotEmpty(t, jobs)
		jobID := jobs[0].ID

		req := httptest.NewRequest(http.MethodDelete, "/api/schedules?id="+jobID, nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		assert.Equal(t, 1, len(engine.ListAllJobs()))
	})
}

func TestBotsStatusAPI(t *testing.T) {
	reg := kakao.NewClientRegistry()

	memStore := kakao.NewMemoryTokenStore(&kakao.TokenInfo{
		AccessToken: "mock-token",
		ExpiresIn:   7200,
		CreatedAt:   time.Now(),
		Scope:       "talk_message",
	})
	client := kakao.NewClient(kakao.ClientConfig{
		ClientID:   "test-client",
		TokenStore: memStore,
	})
	reg.Register("dobby", client, true)

	handler := newBotsStatusHandler(reg, "test-secret-token")

	t.Run("unauthorized access returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/bots/status", nil)
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("returns bot token status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/bots/status", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Bots  []BotStatusItem `json:"bots"`
			Count int             `json:"count"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Count)
		assert.Equal(t, "dobby", resp.Bots[0].Name)
		assert.True(t, resp.Bots[0].HasToken)
		assert.False(t, resp.Bots[0].IsExpired)
	})
}

func TestChatSimulateAPI(t *testing.T) {
	sessionStore := agent.NewMemorySessionStore(10)
	slashReg := GetDefaultSlashCommandRegistry()
	mockAI := ai.NewMockProvider("test-mock")

	handler := newChatSimulateHandler("0xc0de1ab", "", "test-secret-token", nil, nil, nil, nil, mockAI, sessionStore, "You are a bot", slashReg)

	t.Run("unauthorized access returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/agent/chat", strings.NewReader(`{"utterance":"/status"}`))
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("slash command simulation returns reply", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/agent/chat", strings.NewReader(`{"utterance":"/status"}`))
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Success   bool   `json:"success"`
			Utterance string `json:"utterance"`
			ReplyText string `json:"reply_text"`
			LatencyMs int64  `json:"latency_ms"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Contains(t, resp.ReplyText, "서버 상태 보고서")
	})
}
