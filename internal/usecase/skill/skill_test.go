package skill

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/ai"
	"github.com/dh-kam/kakaotalk-dobby/pkg/openbuilder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	var buf bytes.Buffer
	uc := NewSkillServeUseCase()
	go func() {
		_ = uc.Execute(t.Context(), SkillServeRequest{
			ListenAddr: addr,
			ChannelID:  "0xc0de1ab",
			AIProvider: ai.NewMockProvider("mock-llm"),
			Out:        &buf,
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

	postResp, err := http.Post("http://"+addr+"/skill", "application/json", bytes.NewReader(body))
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

	aiPostResp, err := http.Post("http://"+addr+"/skill", "application/json", bytes.NewReader(aiBody))
	require.NoError(t, err)
	defer aiPostResp.Body.Close()

	assert.Equal(t, http.StatusOK, aiPostResp.StatusCode)

	var aiSkillResp openbuilder.SkillResponse
	err = json.NewDecoder(aiPostResp.Body).Decode(&aiSkillResp)
	require.NoError(t, err)
	assert.Equal(t, "2.0", aiSkillResp.Version)
	assert.NotEmpty(t, aiSkillResp.Template.Outputs)
	assert.Contains(t, aiSkillResp.Template.Outputs[0].SimpleText.Text, "AI Mock Reply")
}
