package auth

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dh-kam/kakao-bot/pkg/kakao"
)

// LoginRequest holds parameters for authentication login.
type LoginRequest struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenPath    string
	Code         string
	Scopes       []string
	Manual       bool
	Out          io.Writer
}

// LoginUseCase handles Kakao OAuth2 login flow.
type LoginUseCase struct{}

func NewLoginUseCase() *LoginUseCase {
	return &LoginUseCase{}
}

// Execute performs the login flow.
func (uc *LoginUseCase) Execute(ctx context.Context, req LoginRequest) error {
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
	if len(req.Scopes) == 0 {
		req.Scopes = []string{kakao.ScopeTalkMessage, kakao.ScopeFriends, kakao.ScopeProfile}
	}

	oauthClient := kakao.NewOAuthClient(kakao.OAuthConfig{
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURI:  req.RedirectURI,
	})

	store := kakao.NewFileTokenStore(req.TokenPath)

	code := req.Code
	if code == "" && !req.Manual && strings.HasPrefix(req.RedirectURI, "http://localhost") {
		// Attempt local HTTP server callback flow
		var err error
		code, err = uc.waitForLocalCallback(ctx, oauthClient, req)
		if err != nil {
			fmt.Fprintf(out, "Local callback listener failed (%v). Switching to manual prompt...\n", err)
			code = ""
		}
	}

	if code == "" {
		authURL := oauthClient.GetAuthCodeURL(req.Scopes)
		fmt.Fprintln(out, "Please open the following URL in your browser to authorize:")
		fmt.Fprintf(out, "\n  %s\n\n", authURL)
		fmt.Fprint(out, "Enter the authorization code (or full redirect URL): ")

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read auth code input: %w", err)
		}
		input = strings.TrimSpace(input)
		if strings.Contains(input, "code=") {
			parsedURL, err := url.Parse(input)
			if err == nil {
				code = parsedURL.Query().Get("code")
			}
		}
		if code == "" {
			code = input
		}
	}

	if code == "" {
		return fmt.Errorf("authorization code is empty")
	}

	fmt.Fprintln(out, "Exchanging authorization code for tokens...")
	token, err := oauthClient.ExchangeToken(ctx, code)
	if err != nil {
		return fmt.Errorf("exchange token: %w", err)
	}

	if err := store.Save(ctx, token); err != nil {
		return fmt.Errorf("save token: %w", err)
	}

	fmt.Fprintln(out, "Authentication successful! Token saved to:", req.TokenPath)
	return nil
}

func (uc *LoginUseCase) waitForLocalCallback(ctx context.Context, oauthClient *kakao.OAuthClient, req LoginRequest) (string, error) {
	u, err := url.Parse(req.RedirectURI)
	if err != nil {
		return "", err
	}

	host := u.Host
	if !strings.Contains(host, ":") {
		host = host + ":80"
	}

	listener, err := net.Listen("tcp", host)
	if err != nil {
		return "", err
	}
	defer listener.Close()

	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != u.Path {
				http.NotFound(w, r)
				return
			}

			q := r.URL.Query()
			if errParam := q.Get("error"); errParam != "" {
				desc := q.Get("error_description")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(fmt.Sprintf("Kakao Login Error: %s (%s)", errParam, desc)))
				errChan <- fmt.Errorf("kakao login error: %s (%s)", errParam, desc)
				return
			}

			code := q.Get("code")
			if code == "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("No code parameter found."))
				errChan <- fmt.Errorf("no authorization code in callback")
				return
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Kakao Bot Login</title></head><body style="font-family: sans-serif; text-align: center; padding-top: 50px;"><h2>Authentication Successful!</h2><p>You can close this tab and return to the terminal.</p></body></html>`))
			codeChan <- code
		}),
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	authURL := oauthClient.GetAuthCodeURL(req.Scopes)
	fmt.Fprintln(req.Out, "Opening browser for Kakao authorization...")
	fmt.Fprintf(req.Out, "\n  %s\n\n", authURL)
	fmt.Fprintln(req.Out, "Waiting for callback on", req.RedirectURI, "...")

	select {
	case code := <-codeChan:
		_ = server.Shutdown(context.Background())
		return code, nil
	case err := <-errChan:
		_ = server.Shutdown(context.Background())
		return "", err
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return "", ctx.Err()
	case <-time.After(3 * time.Minute):
		_ = server.Shutdown(context.Background())
		return "", fmt.Errorf("timeout waiting for browser login callback")
	}
}

// StatusRequest holds parameters for status checking.
type StatusRequest struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenPath    string
	Out          io.Writer
}

// StatusUseCase checks current authentication state and Kakao profile.
type StatusUseCase struct{}

func NewStatusUseCase() *StatusUseCase {
	return &StatusUseCase{}
}

func (uc *StatusUseCase) Execute(ctx context.Context, req StatusRequest) error {
	out := req.Out
	if out == nil {
		out = os.Stdout
	}

	store := kakao.NewFileTokenStore(req.TokenPath)
	token, err := store.Load(ctx)
	if err != nil {
		if errors.Is(err, kakao.ErrTokenNotFound) {
			return fmt.Errorf("no saved token found at %q. Run 'kakaobot auth login' first", req.TokenPath)
		}
		return fmt.Errorf("load token: %w", err)
	}

	oauthClient := kakao.NewOAuthClient(kakao.OAuthConfig{
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURI:  req.RedirectURI,
	})

	client := kakao.NewClient(kakao.ClientConfig{
		OAuthClient: oauthClient,
		TokenStore:  store,
	})

	profile, err := client.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch user profile: %w", err)
	}

	fmt.Fprintln(out, "KakaoTalk Authentication Status:")
	fmt.Fprintf(out, "  User ID:         %d\n", profile.ID)
	if profile.KakaoAccount != nil && profile.KakaoAccount.Profile != nil {
		fmt.Fprintf(out, "  Nickname:        %s\n", profile.KakaoAccount.Profile.Nickname)
	}
	if profile.KakaoAccount != nil && profile.KakaoAccount.Email != "" {
		fmt.Fprintf(out, "  Email:           %s\n", profile.KakaoAccount.Email)
	}
	fmt.Fprintf(out, "  Token File:      %s\n", req.TokenPath)
	fmt.Fprintf(out, "  Token Scope:     %s\n", token.Scope)
	fmt.Fprintf(out, "  Access Expired:  %v\n", token.IsExpired())
	fmt.Fprintf(out, "  Refresh Expired: %v\n", token.IsRefreshTokenExpired())

	return nil
}

// RefreshRequest holds parameters for manual token refresh.
type RefreshRequest struct {
	ClientID     string
	ClientSecret string
	TokenPath    string
	Out          io.Writer
}

// RefreshUseCase forces token refresh.
type RefreshUseCase struct{}

func NewRefreshUseCase() *RefreshUseCase {
	return &RefreshUseCase{}
}

func (uc *RefreshUseCase) Execute(ctx context.Context, req RefreshRequest) error {
	out := req.Out
	if out == nil {
		out = os.Stdout
	}

	if req.ClientID == "" {
		return fmt.Errorf("client ID (REST API Key) is required")
	}

	store := kakao.NewFileTokenStore(req.TokenPath)
	token, err := store.Load(ctx)
	if err != nil {
		return fmt.Errorf("load token: %w", err)
	}

	if token.RefreshToken == "" {
		return fmt.Errorf("refresh token is empty; please run 'kakaobot auth login' again")
	}

	oauthClient := kakao.NewOAuthClient(kakao.OAuthConfig{
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
	})

	refreshed, err := oauthClient.RefreshToken(ctx, token.RefreshToken)
	if err != nil {
		return fmt.Errorf("refresh token: %w", err)
	}

	if err := store.Save(ctx, refreshed); err != nil {
		return fmt.Errorf("save refreshed token: %w", err)
	}

	fmt.Fprintln(out, "Access token refreshed successfully.")
	return nil
}
