package kakao

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageService_UploadAndScrapAndDelete(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/v2/api/talk/message/image/upload" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"infos": {
					"original": {
						"url": "https://k.kakaocdn.net/image.png",
						"length": 1024,
						"width": 640,
						"height": 480
					}
				}
			}`))
			return
		}

		if r.URL.Path == "/v2/api/talk/message/image/scrap" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"infos": {
					"original": {
						"url": "https://k.kakaocdn.net/scraped.png",
						"length": 2048,
						"width": 800,
						"height": 600
					}
				}
			}`))
			return
		}

		if r.URL.Path == "/v2/api/talk/message/image/delete" {
			assert.Equal(t, http.MethodDelete, r.Method)
			w.WriteHeader(http.StatusOK)
			return
		}

		http.NotFound(w, r)
	}))
	defer ts.Close()

	svc := NewStorageService(ts.URL, nil, func(ctx context.Context) (string, error) {
		return "tok", nil
	})

	uploadRes, err := svc.UploadImage(context.Background(), strings.NewReader("fake image bytes"), "test.png")
	require.NoError(t, err)
	assert.Equal(t, "https://k.kakaocdn.net/image.png", uploadRes.Infos.Original.URL)

	scrapRes, err := svc.ScrapImage(context.Background(), "https://remote.com/pic.jpg")
	require.NoError(t, err)
	assert.Equal(t, "https://k.kakaocdn.net/scraped.png", scrapRes.Infos.Original.URL)

	err = svc.DeleteImage(context.Background(), "https://k.kakaocdn.net/image.png")
	require.NoError(t, err)
}
