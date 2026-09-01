package webhook

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
)

// maxRequestBodyBytes caps incoming webhook payloads to block memory exhaustion.
const maxRequestBodyBytes = 1 << 20

// ServeRequest holds options for starting the webhook relay server.
type ServeRequest struct {
	ListenAddr   string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenPath    string
	SecretToken  string
	Out          io.Writer
}

// WebhookPayload represents incoming webhook payload.
type WebhookPayload struct {
	Text          string   `json:"text"`
	Message       string   `json:"message,omitempty"`
	URL           string   `json:"url,omitempty"`
	ButtonTitle   string   `json:"button_title,omitempty"`
	ReceiverUUIDs []string `json:"receiver_uuids,omitempty"`
}

// ServeUseCase starts a webhook HTTP server that relays incoming messages to KakaoTalk.
type ServeUseCase struct{}

func NewServeUseCase() *ServeUseCase {
	return &ServeUseCase{}
}

func (uc *ServeUseCase) Execute(ctx context.Context, req ServeRequest) error {
	out := req.Out
	if out == nil {
		out = os.Stdout
	}

	tokenStore := kakao.NewFileTokenStore(req.TokenPath)
	client := kakao.NewClient(kakao.ClientConfig{
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURI:  req.RedirectURI,
		TokenStore:   tokenStore,
	})

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		if req.SecretToken != "" {
			authHeader := r.Header.Get("X-Webhook-Token")
			if subtle.ConstantTimeCompare([]byte(authHeader), []byte(req.SecretToken)) != 1 {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var payload WebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			payload.Text = strings.TrimSpace(string(body))
		}

		msgText := payload.Text
		if msgText == "" {
			msgText = payload.Message
		}

		if msgText == "" {
			http.Error(w, "text or message field is required", http.StatusBadRequest)
			return
		}

		msgReq := kakao.TextMessageRequest{
			Text:        msgText,
			WebURL:      payload.URL,
			ButtonTitle: payload.ButtonTitle,
		}

		var sendErr error
		if len(payload.ReceiverUUIDs) > 0 {
			_, sendErr = client.Friends().SendText(r.Context(), payload.ReceiverUUIDs, msgReq)
		} else {
			sendErr = client.Memo().SendText(r.Context(), msgReq)
		}

		if sendErr != nil {
			http.Error(w, fmt.Sprintf("Failed to send Kakao message: %v", sendErr), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	})

	server := &http.Server{
		Addr:         req.ListenAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	fmt.Fprintf(out, "Starting Kakao webhook server on %s...\n", req.ListenAddr)
	fmt.Fprintf(out, "  - Health check: http://%s/healthz\n", req.ListenAddr)
	fmt.Fprintf(out, "  - Webhook POST: http://%s/webhook\n", req.ListenAddr)
	if req.SecretToken != "" {
		fmt.Fprintln(out, "  - Authentication: Enabled (X-Webhook-Token required)")
	} else if !strings.HasPrefix(req.ListenAddr, "127.0.0.1") && !strings.HasPrefix(req.ListenAddr, "localhost") {
		fmt.Fprintln(out, "  ⚠️ WARNING: Webhook server is bound to an open address without secret-token authentication.")
	}

	errChan := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Fprintln(out, "\nShutting down webhook server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	}
}
