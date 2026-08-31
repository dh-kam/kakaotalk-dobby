package webhook

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

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

func startWebhookServer(t *testing.T, addr, secretToken string) {
	t.Helper()

	var buf bytes.Buffer
	uc := NewServeUseCase()
	go func() {
		_ = uc.Execute(t.Context(), ServeRequest{
			ListenAddr:  addr,
			TokenPath:   "non-existent.json",
			SecretToken: secretToken,
			Out:         &buf,
		})
	}()
}

func TestWebhookServer_Healthz(t *testing.T) {
	addr := "127.0.0.1:18081"
	startWebhookServer(t, addr, "")
	waitForServer(t, "http://"+addr+"/healthz")

	resp, err := http.Get("http://" + addr + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWebhookServer_BodyTooLarge(t *testing.T) {
	addr := "127.0.0.1:18083"
	startWebhookServer(t, addr, "")
	waitForServer(t, "http://"+addr+"/healthz")

	oversized := strings.Repeat("a", maxRequestBodyBytes+1)
	resp, err := http.Post("http://"+addr+"/webhook", "application/json", strings.NewReader(oversized))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
}

func TestWebhookServer_SecretToken(t *testing.T) {
	addr := "127.0.0.1:18084"
	startWebhookServer(t, addr, "secret-token")
	waitForServer(t, "http://"+addr+"/healthz")

	postWithToken := func(token string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/webhook", strings.NewReader(`{}`))
		require.NoError(t, err)
		req.Header.Set("X-Webhook-Token", token)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		t.Cleanup(func() { resp.Body.Close() })
		return resp
	}

	assert.Equal(t, http.StatusUnauthorized, postWithToken("wrong").StatusCode)
	assert.Equal(t, http.StatusUnauthorized, postWithToken("").StatusCode)

	// A valid token passes authentication and reaches payload validation.
	assert.Equal(t, http.StatusBadRequest, postWithToken("secret-token").StatusCode)
}
