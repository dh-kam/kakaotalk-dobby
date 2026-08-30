package kakao

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserService_GetMe(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/user/me", r.URL.Path)
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": 123456,
			"kakao_account": {
				"profile": {
					"nickname": "Tester"
				},
				"email": "test@example.com"
			}
		}`))
	}))
	defer ts.Close()

	svc := NewUserService(ts.URL, nil, func(ctx context.Context) (string, error) {
		return "tok", nil
	})

	profile, err := svc.GetMe(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(123456), profile.ID)
	assert.Equal(t, "Tester", profile.KakaoAccount.Profile.Nickname)
	assert.Equal(t, "test@example.com", profile.KakaoAccount.Email)
}
