package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/dh-kam/kakao-bot/pkg/openbuilder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillServer_ProcessUtterance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var buf bytes.Buffer
	uc := NewSkillServeUseCase()

	go func() {
		_ = uc.Execute(ctx, SkillServeRequest{
			ListenAddr: "127.0.0.1:18082",
			ChannelID:  "0xc0de1ab",
			Out:        &buf,
		})
	}()

	var resp *http.Response
	var err error
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		resp, err = http.Get("http://127.0.0.1:18082/healthz")
		if err == nil {
			break
		}
	}
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Test POST /skill
	reqPayload := openbuilder.SkillPayload{
		UserRequest: openbuilder.UserRequest{
			Utterance: "도움말",
			User: openbuilder.ChatUser{
				ID: "test-user",
			},
		},
	}
	body, _ := json.Marshal(reqPayload)

	postResp, err := http.Post("http://127.0.0.1:18082/skill", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer postResp.Body.Close()

	assert.Equal(t, http.StatusOK, postResp.StatusCode)

	var skillResp openbuilder.SkillResponse
	err = json.NewDecoder(postResp.Body).Decode(&skillResp)
	require.NoError(t, err)
	assert.Equal(t, "2.0", skillResp.Version)
	assert.NotEmpty(t, skillResp.Template.Outputs)
	assert.Contains(t, skillResp.Template.Outputs[0].SimpleText.Text, "0xc0de1ab")
}
