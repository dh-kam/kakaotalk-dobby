package skill

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/academy"
	"github.com/dh-kam/kakaotalk-dobby/pkg/agent"
	"github.com/dh-kam/kakaotalk-dobby/pkg/ai"
	"github.com/dh-kam/kakaotalk-dobby/pkg/openbuilder"
	"github.com/dh-kam/kakaotalk-dobby/pkg/scheduler"
	"github.com/dh-kam/kakaotalk-dobby/pkg/school"
)

type chatSimulateRequest struct {
	Utterance string `json:"utterance"`
	UserID    string `json:"user_id,omitempty"`
}

func newChatSimulateHandler(channelID, dataDir, adminToken string, busSvc *academy.Service, schoolSvc *school.Service, schedEngine *scheduler.Engine, botAgent agent.Agent, aiProvider ai.Provider, sessionStore agent.SessionStore, systemPrompt string, slashRegistry *SlashCommandRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		if !checkAdminAuth(r, adminToken) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "payload too large or read failed"})
			return
		}
		defer r.Body.Close()

		var req chatSimulateRequest
		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid JSON: " + err.Error()})
			return
		}

		utterance := strings.TrimSpace(req.Utterance)
		if utterance == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "utterance is required"})
			return
		}

		userID := strings.TrimSpace(req.UserID)
		if userID == "" {
			userID = "web-simulator"
		}

		start := time.Now()
		resp := processUtterance(r.Context(), utterance, userID, channelID, dataDir, adminToken, busSvc, schoolSvc, schedEngine, botAgent, aiProvider, sessionStore, systemPrompt, slashRegistry)
		latency := time.Since(start).Milliseconds()

		if resp != nil {
			resp.ValidateAndNormalize()
		}

		plainText := extractPlainText(resp)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":    true,
			"utterance":  utterance,
			"reply_text": plainText,
			"response":   resp,
			"latency_ms": latency,
		})
	}
}

func extractPlainText(resp *openbuilder.SkillResponse) string {
	if resp == nil || len(resp.Template.Outputs) == 0 {
		return ""
	}

	var parts []string
	for _, out := range resp.Template.Outputs {
		if out.SimpleText != nil && out.SimpleText.Text != "" {
			parts = append(parts, out.SimpleText.Text)
		}
		if out.BasicCard != nil {
			var cardParts []string
			if out.BasicCard.Title != "" {
				cardParts = append(cardParts, out.BasicCard.Title)
			}
			if out.BasicCard.Description != "" {
				cardParts = append(cardParts, out.BasicCard.Description)
			}
			if len(cardParts) > 0 {
				parts = append(parts, strings.Join(cardParts, "\n"))
			}
		}
	}

	return strings.Join(parts, "\n\n")
}
