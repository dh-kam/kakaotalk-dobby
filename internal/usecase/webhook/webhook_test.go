package webhook

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookServer_Healthz(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var buf bytes.Buffer
	uc := NewServeUseCase()

	go func() {
		_ = uc.Execute(ctx, ServeRequest{
			ListenAddr: "127.0.0.1:18081",
			TokenPath:  "non-existent.json",
			Out:        &buf,
		})
	}()

	var resp *http.Response
	var err error
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		resp, err = http.Get("http://127.0.0.1:18081/healthz")
		if err == nil {
			break
		}
	}
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
