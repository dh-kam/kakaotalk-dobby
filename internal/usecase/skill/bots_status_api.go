package skill

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
)

// BotStatusItem represents runtime status of a registered bot and its OAuth token.
type BotStatusItem struct {
	Name                string `json:"name"`
	IsDefault           bool   `json:"is_default"`
	HasToken            bool   `json:"has_token"`
	IsExpired           bool   `json:"is_expired"`
	RefreshTokenExpired bool   `json:"refresh_token_expired"`
	ExpiresIn           int64  `json:"expires_in"`
	CreatedAt           string `json:"created_at,omitempty"`
	Scope               string `json:"scope,omitempty"`
}

func newBotsStatusHandler(reg kakao.ClientRegistry, adminToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		if !checkAdminAuth(r, adminToken) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
			return
		}

		if reg == nil {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"bots":  []BotStatusItem{},
				"count": 0,
			})
			return
		}

		infoList := reg.Info()
		items := make([]BotStatusItem, 0, len(infoList))

		for _, info := range infoList {
			item := BotStatusItem{
				Name:      info.Name,
				IsDefault: info.IsDefault,
			}

			if client, err := reg.Get(info.Name); err == nil && client != nil {
				if store := client.GetTokenStore(); store != nil {
					if tok, err := store.Load(r.Context()); err == nil && tok != nil {
						item.HasToken = true
						item.IsExpired = tok.IsExpired()
						item.RefreshTokenExpired = tok.IsRefreshTokenExpired()
						item.ExpiresIn = tok.ExpiresIn
						if !tok.CreatedAt.IsZero() {
							item.CreatedAt = tok.CreatedAt.Format(time.RFC3339)
						}
						item.Scope = tok.Scope
					}
				}
			}

			items = append(items, item)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"bots":  items,
			"count": len(items),
		})
	}
}
