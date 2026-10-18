package skill

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/dh-kam/kakaotalk-dobby/internal/usecase/notify"
	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
)

func newNotifyHandler(reg kakao.ClientRegistry, adminToken, defaultChannelID string) http.HandlerFunc {
	var uc *notify.NotifyUseCase
	if reg != nil {
		uc = notify.NewNotifyUseCase(reg, notify.NewDefaultChannelSender(defaultChannelID))
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		if !checkAdminAuth(r, adminToken) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "unauthorized"})
			return
		}

		if uc == nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "kakao client registry is not configured"})
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "payload too large or read failed"})
			return
		}
		defer r.Body.Close()

		var req notify.SendMessageRequest
		if err := json.Unmarshal(body, &req); err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "invalid JSON payload: " + err.Error()})
			return
		}

		resp, err := uc.Send(r.Context(), req)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err != nil {
			if errors.Is(err, notify.ErrInvalidRequest) {
				w.WriteHeader(http.StatusBadRequest)
			} else if errors.Is(err, kakao.ErrBotNotFound) {
				w.WriteHeader(http.StatusNotFound)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func newBotsListHandler(reg kakao.ClientRegistry, adminToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		if !checkAdminAuth(r, adminToken) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if reg == nil {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"bots": []any{}, "count": 0})
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"bots":  reg.Info(),
			"count": reg.Count(),
		})
	}
}

func newFriendsListHandler(reg kakao.ClientRegistry, adminToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		if !checkAdminAuth(r, adminToken) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if reg == nil {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"elements": []any{}, "total_count": 0})
			return
		}

		botName := r.URL.Query().Get("bot")
		client, err := reg.Get(botName)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}

		friendsResp, err := client.Friends().GetFriends(r.Context(), kakao.FriendsQueryOptions{Limit: 100})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": fmt.Sprintf("failed to get friends: %v", err)})
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(friendsResp)
	}
}

