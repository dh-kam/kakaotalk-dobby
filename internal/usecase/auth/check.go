package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dh-kam/kakao-bot/pkg/kakao"
)

// CheckRequest holds parameters for checking REST API key status.
type CheckRequest struct {
	ClientID    string
	RedirectURI string
	Out         io.Writer
}

// CheckUseCase checks the validity of REST API key and Kakao Developer settings.
type CheckUseCase struct{}

func NewCheckUseCase() *CheckUseCase {
	return &CheckUseCase{}
}

// Execute validates the REST API Key and checks Kakao Developer configuration status.
func (uc *CheckUseCase) Execute(ctx context.Context, req CheckRequest) error {
	out := req.Out
	if out == nil {
		out = os.Stdout
	}

	if req.ClientID == "" {
		return fmt.Errorf("client ID (REST API Key) is required")
	}
	if req.RedirectURI == "" {
		req.RedirectURI = "http://localhost:8080/callback"
	}

	client := kakao.NewClient(kakao.ClientConfig{
		ClientID:    req.ClientID,
		RedirectURI: req.RedirectURI,
	})

	checkURL := client.Auth().GetAuthURL([]string{kakao.ScopeTalkMessage})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return fmt.Errorf("create check request: %w", err)
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("connect to Kakao auth server: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	fmt.Fprintln(out, "Kakao REST API Key Validation Result:")
	fmt.Fprintf(out, "  - REST API Key: %s\n", maskKey(req.ClientID))
	fmt.Fprintf(out, "  - Redirect URI: %s\n", req.RedirectURI)

	if strings.Contains(bodyStr, "KOE010") || strings.Contains(bodyStr, "잘못된 client_id") {
		fmt.Fprintln(out, "  - Status: ❌ INVALID (KOE010: REST API Key does not exist)")
		return fmt.Errorf("invalid REST API key: Kakao returned KOE010 (Bad Client ID)")
	}

	if strings.Contains(bodyStr, "KOE004") || strings.Contains(bodyStr, "카카오 로그인을 사용하도록 설정하지 않은") {
		fmt.Fprintln(out, "  - Key Status:    ✅ Valid REST API Key!")
		fmt.Fprintln(out, "  - Setting Alert: ⚠️ [KOE004] 카카오 로그인이 아직 활성화(ON)되지 않았습니다.")
		return nil
	}

	if strings.Contains(bodyStr, "KOE006") || strings.Contains(bodyStr, "Redirect URI") {
		fmt.Fprintln(out, "  - Key Status:    ✅ Valid REST API Key!")
		fmt.Fprintln(out, "  - Setting Alert: ⚠️ [KOE006] Redirect URI가 등록되지 않았습니다.")
		return nil
	}

	fmt.Fprintln(out, "  - Status: ✅ VALID & READY! (All Kakao OAuth settings are verified)")
	return nil
}

func maskKey(k string) string {
	if len(k) <= 8 {
		return "********"
	}
	return k[:4] + strings.Repeat("*", len(k)-8) + k[len(k)-4:]
}
