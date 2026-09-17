package skill

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/academy"
	"github.com/dh-kam/kakaotalk-dobby/pkg/ai"
	"github.com/dh-kam/kakaotalk-dobby/pkg/openbuilder"
	"github.com/dh-kam/kakaotalk-dobby/pkg/school"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSkillToken = "test-only-skill-secret"

func postSkillRequest(t *testing.T, url string, body io.Reader) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Skill-Token", testSkillToken)
	return http.DefaultClient.Do(req)
}

func waitForServer(t *testing.T, url string) {
	t.Helper()

	var lastErr error
	for range 20 {
		time.Sleep(50 * time.Millisecond)
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		lastErr = err
	}
	t.Fatalf("server did not start: %v", lastErr)
}

func TestSkillServer_ProcessUtterance(t *testing.T) {
	addr := "127.0.0.1:18082"

	busSvc := academy.NewService()
	_ = busSvc.LoadFromDir("../../data/schedules")
	schoolSvc := school.NewService()
	_ = schoolSvc.LoadFromDir("../../data/schedules")

	var buf bytes.Buffer
	uc := NewSkillServeUseCase()
	go func() {
		_ = uc.Execute(t.Context(), SkillServeRequest{
			SkillToken:    testSkillToken,
			ListenAddr:    addr,
			ChannelID:     "0xc0de1ab",
			AIProvider:    ai.NewMockProvider("mock-llm"),
			BusService:    busSvc,
			SchoolService: schoolSvc,
			DataDir:       "../../data/schedules",
			Out:           &buf,
		})
	}()
	waitForServer(t, "http://"+addr+"/healthz")

	// Test built-in command
	reqPayload := openbuilder.SkillPayload{
		UserRequest: openbuilder.UserRequest{
			Utterance: "도움말",
			User: openbuilder.ChatUser{
				ID: "test-user",
			},
		},
	}
	body, _ := json.Marshal(reqPayload)

	postResp, err := postSkillRequest(t, "http://"+addr+"/skill", bytes.NewReader(body))
	require.NoError(t, err)
	defer postResp.Body.Close()

	assert.Equal(t, http.StatusOK, postResp.StatusCode)

	var skillResp openbuilder.SkillResponse
	err = json.NewDecoder(postResp.Body).Decode(&skillResp)
	require.NoError(t, err)
	assert.Equal(t, "2.0", skillResp.Version)
	assert.NotEmpty(t, skillResp.Template.Outputs)
	assert.Contains(t, skillResp.Template.Outputs[0].SimpleText.Text, "0xc0de1ab")

	// Test AI Question Answering
	aiReqPayload := openbuilder.SkillPayload{
		UserRequest: openbuilder.UserRequest{
			Utterance: "Go 언어로 챗봇 만드는 법 알려줘",
			User: openbuilder.ChatUser{
				ID: "test-user",
			},
		},
	}
	aiBody, _ := json.Marshal(aiReqPayload)

	aiPostResp, err := postSkillRequest(t, "http://"+addr+"/skill", bytes.NewReader(aiBody))
	require.NoError(t, err)
	defer aiPostResp.Body.Close()

	assert.Equal(t, http.StatusOK, aiPostResp.StatusCode)

	var aiSkillResp openbuilder.SkillResponse
	err = json.NewDecoder(aiPostResp.Body).Decode(&aiSkillResp)
	require.NoError(t, err)
	assert.Equal(t, "2.0", aiSkillResp.Version)
	assert.NotEmpty(t, aiSkillResp.Template.Outputs)
	assert.Contains(t, aiSkillResp.Template.Outputs[0].SimpleText.Text, "AI Mock Reply")

	// Test Reset Command
	resetPayload := openbuilder.SkillPayload{
		UserRequest: openbuilder.UserRequest{
			Utterance: "대화 초기화",
			User: openbuilder.ChatUser{
				ID: "test-user",
			},
		},
	}
	resetBody, _ := json.Marshal(resetPayload)
	resetPostResp, err := postSkillRequest(t, "http://"+addr+"/skill", bytes.NewReader(resetBody))
	require.NoError(t, err)
	defer resetPostResp.Body.Close()

	var resetSkillResp openbuilder.SkillResponse
	err = json.NewDecoder(resetPostResp.Body).Decode(&resetSkillResp)
	require.NoError(t, err)
	assert.Contains(t, resetSkillResp.Template.Outputs[0].SimpleText.Text, "초기화했습니다")

	// Test School Timetable Fast Path
	schoolPayload := openbuilder.SkillPayload{
		UserRequest: openbuilder.UserRequest{
			Utterance: "월요일 학교 시간표 알려줘",
			User: openbuilder.ChatUser{
				ID: "test-user",
			},
		},
	}
	schoolBody, _ := json.Marshal(schoolPayload)
	schoolPostResp, err := postSkillRequest(t, "http://"+addr+"/skill", bytes.NewReader(schoolBody))
	require.NoError(t, err)
	defer schoolPostResp.Body.Close()

	var schoolSkillResp openbuilder.SkillResponse
	err = json.NewDecoder(schoolPostResp.Body).Decode(&schoolSkillResp)
	require.NoError(t, err)
	assert.NotEmpty(t, schoolSkillResp.Template.Outputs)
	assert.Contains(t, schoolSkillResp.Template.Outputs[0].SimpleText.Text, "월요일")

	// Test Bus Fast Path
	busPayload := openbuilder.SkillPayload{
		UserRequest: openbuilder.UserRequest{
			Utterance: "우미린 2차 버스 몇 시에 와?",
			User: openbuilder.ChatUser{
				ID: "test-user",
			},
		},
	}
	busBody, _ := json.Marshal(busPayload)
	busPostResp, err := postSkillRequest(t, "http://"+addr+"/skill", bytes.NewReader(busBody))
	require.NoError(t, err)
	defer busPostResp.Body.Close()

	var busSkillResp openbuilder.SkillResponse
	err = json.NewDecoder(busPostResp.Body).Decode(&busSkillResp)
	require.NoError(t, err)
	assert.NotEmpty(t, busSkillResp.Template.Outputs)
}
