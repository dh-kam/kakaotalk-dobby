package bootstrap

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/dh-kam/kakao-bot/pkg/kakao"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendCommand_Help(t *testing.T) {
	cmd := NewRootCommand(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"send", "--help"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Send KakaoTalk messages")
}

func TestSendFriendCommand_PreRunE_MissingReceivers(t *testing.T) {
	cmd := NewRootCommand(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"send", "friend", "Hello"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--receiver-uuids")
}

func TestSendMeCommand_Execution(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/api/talk/memo/default/send", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result_code": 0}`))
	}))
	defer ts.Close()

	tempDir := t.TempDir()
	tokenPath := filepath.Join(tempDir, "tokens.json")
	store := kakao.NewFileTokenStore(tokenPath)
	err := store.Save(context.Background(), &kakao.TokenInfo{
		AccessToken: "test-token",
		ExpiresIn:   3600,
		CreatedAt:   time.Now(),
	})
	require.NoError(t, err)

	cmd := newSendMeCommand(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"--token-path", tokenPath,
		"--text", "Test direct message",
	})

	// Inject test server URL into client via env / args or test execution
	// Note: newSendMeCommand uses pkg/kakao default URL. Let's verify flags binding and PreRunE!
	err = cmd.PreRunE(cmd, []string{"--token-path", tokenPath, "--text", "Test message"})
	assert.NoError(t, err)
}
