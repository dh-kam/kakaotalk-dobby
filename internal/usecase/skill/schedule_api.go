package skill

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/scheduler"
)

type createScheduleRequest struct {
	UserID    string            `json:"user_id,omitempty"`
	Title     string            `json:"title"`
	Message   string            `json:"message"`
	Type      string            `json:"type,omitempty"` // "once" | "recurring"
	ExecuteAt string            `json:"execute_at,omitempty"`
	CronExpr  string            `json:"cron_expr,omitempty"`
	Bot       string            `json:"bot,omitempty"`
	Payload   map[string]string `json:"payload,omitempty"`
}

func newScheduleHandler(schedEngine *scheduler.Engine, adminToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		if !checkAdminAuth(r, adminToken) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
			return
		}

		if schedEngine == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "scheduler engine is not configured"})
			return
		}

		switch r.Method {
		case http.MethodGet:
			userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
			var jobs []*scheduler.Job
			if userID != "" {
				jobs = schedEngine.ListJobs(userID)
			} else {
				jobs = schedEngine.ListAllJobs()
			}
			if jobs == nil {
				jobs = []*scheduler.Job{}
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jobs":  jobs,
				"count": len(jobs),
			})

		case http.MethodPost:
			r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "payload too large or read failed"})
				return
			}
			defer r.Body.Close()

			var req createScheduleRequest
			if err := json.Unmarshal(body, &req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid JSON: " + err.Error()})
				return
			}

			if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Message) == "" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "title and message are required"})
				return
			}

			payload := req.Payload
			if payload == nil {
				payload = make(map[string]string)
			}
			if req.Bot != "" {
				payload["bot"] = strings.ToLower(strings.TrimSpace(req.Bot))
			}

			userID := req.UserID
			if userID == "" {
				userID = "admin-web"
			}

			var job *scheduler.Job
			if strings.EqualFold(req.Type, "recurring") || req.CronExpr != "" {
				if req.CronExpr == "" {
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]any{"error": "cron_expr is required for recurring schedule"})
					return
				}
				job, err = schedEngine.ScheduleRecurring(userID, req.Title, req.Message, req.CronExpr, payload)
			} else {
				execTime, parseErr := parseScheduleTime(req.ExecuteAt, schedEngine.Location())
				if parseErr != nil {
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid execute_at: " + parseErr.Error()})
					return
				}
				job, err = schedEngine.ScheduleOnce(userID, req.Title, req.Message, execTime, payload)
			}

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
				return
			}

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"job":     job,
			})

		case http.MethodDelete:
			id := strings.TrimSpace(r.URL.Query().Get("id"))
			if id == "" {
				// Also allow JSON body with {"id": "..."}
				var req struct {
					ID string `json:"id"`
				}
				if body, err := io.ReadAll(r.Body); err == nil && len(body) > 0 {
					_ = json.Unmarshal(body, &req)
					id = strings.TrimSpace(req.ID)
				}
			}

			if id == "" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "job id is required"})
				return
			}

			if err := schedEngine.DeleteJob(id); err != nil {
				if errors.Is(err, scheduler.ErrJobNotFound) {
					w.WriteHeader(http.StatusNotFound)
				} else {
					w.WriteHeader(http.StatusInternalServerError)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
				return
			}

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"deleted": id,
			})

		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	}
}

func parseScheduleTime(raw string, loc *time.Location) (time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, errors.New("execute_at is empty")
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}

	for _, fmtStr := range formats {
		if t, err := time.ParseInLocation(fmtStr, s, loc); err == nil {
			return t, nil
		}
	}

	// Try duration from now (e.g. "10m", "1h")
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().In(loc).Add(d), nil
	}

	return time.Time{}, errors.New("unsupported time format, expected RFC3339 or 'YYYY-MM-DD HH:MM'")
}
