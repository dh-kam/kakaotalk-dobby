package kakao

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoService_SendTextAndFeed(t *testing.T) {
	var receivedBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/api/talk/memo/default/send", r.URL.Path)
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))

		assert.NoError(t, r.ParseForm())
		receivedBody = r.Form.Get("template_object")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result_code": 0}`))
	}))
	defer ts.Close()

	svc := NewMemoService(ts.URL, nil, func(ctx context.Context) (string, error) {
		return "tok", nil
	})

	err := svc.SendText(context.Background(), TextMessageRequest{
		Text:        "Hello Text",
		WebURL:      "https://example.com",
		ButtonTitle: "Click",
	})
	require.NoError(t, err)
	assert.Contains(t, receivedBody, "Hello Text")

	feed := NewFeedTemplate("Feed Title", "Feed Desc", "https://img.com", "https://link.com", "Open")
	err = svc.SendFeed(context.Background(), *feed)
	require.NoError(t, err)
	assert.Contains(t, receivedBody, "Feed Title")
}
