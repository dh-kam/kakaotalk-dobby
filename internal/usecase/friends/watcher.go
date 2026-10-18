package friends

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
)

// DefaultFriendsAlertStatePath returns ~/.config/kakao-bot/friends_alert_state.json.
func DefaultFriendsAlertStatePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "friends_alert_state.json"
	}
	return filepath.Join(home, ".config", "kakao-bot", "friends_alert_state.json")
}

// AlertRecord stores information about an unknown friend that has been alerted.
type AlertRecord struct {
	UUID      string    `json:"uuid"`
	Nickname  string    `json:"nickname"`
	KakaoID   int64     `json:"kakao_id"`
	AlertedAt time.Time `json:"alerted_at"`
}

// AlertState stores history of alerted unknown friends to prevent notification spam.
type AlertState struct {
	Alerted map[string]AlertRecord `json:"alerted"`
}

// UnknownFriendDetection represents the check result for an unknown friend.
type UnknownFriendDetection struct {
	Friend     kakao.Friend
	IsNewAlert bool
	AlertSent  bool
	AlertError error
}

// FriendWatcherConfig configures the FriendWatcher.
type FriendWatcherConfig struct {
	Client         kakao.Client
	Directory      FriendDirectory
	StatePath      string
	AdminEmail     string        // Default: "inzero@hanmail.net"
	Interval       time.Duration // Default: 1 hour
	AlertOnUnknown bool          // Default: true
	Out            io.Writer     // Optional output writer for logging
}

// FriendWatcher periodically scans for unknown friends and notifies the admin.
type FriendWatcher struct {
	client         kakao.Client
	directory      FriendDirectory
	statePath      string
	adminEmail     string
	interval       time.Duration
	alertOnUnknown bool
	out            io.Writer

	mu        sync.Mutex
	state     AlertState
	cancel    context.CancelFunc
	done      chan struct{}
	isRunning bool
}

// NewFriendWatcher constructs a FriendWatcher.
func NewFriendWatcher(cfg FriendWatcherConfig) *FriendWatcher {
	if cfg.StatePath == "" {
		cfg.StatePath = DefaultFriendsAlertStatePath()
	}
	if cfg.AdminEmail == "" {
		cfg.AdminEmail = "inzero@hanmail.net"
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 1 * time.Hour
	}
	if cfg.Out == nil {
		cfg.Out = os.Stdout
	}

	w := &FriendWatcher{
		client:         cfg.Client,
		directory:      cfg.Directory,
		statePath:      cfg.StatePath,
		adminEmail:     cfg.AdminEmail,
		interval:       cfg.Interval,
		alertOnUnknown: cfg.AlertOnUnknown,
		out:            cfg.Out,
		state: AlertState{
			Alerted: make(map[string]AlertRecord),
		},
		done: make(chan struct{}),
	}

	_ = w.loadState()
	return w
}

func (w *FriendWatcher) loadState() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	data, err := os.ReadFile(w.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			w.state = AlertState{Alerted: make(map[string]AlertRecord)}
			return nil
		}
		return err
	}

	var state AlertState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	if state.Alerted == nil {
		state.Alerted = make(map[string]AlertRecord)
	}
	w.state = state
	return nil
}

func (w *FriendWatcher) saveStateUnlocked() error {
	if w.statePath == "" {
		return nil
	}

	dir := filepath.Dir(w.statePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	bytes, err := json.MarshalIndent(w.state, "", "  ")
	if err != nil {
		return err
	}

	tmpFile := w.statePath + ".tmp"
	if err := os.WriteFile(tmpFile, bytes, 0600); err != nil {
		return err
	}

	return os.Rename(tmpFile, w.statePath)
}

// CheckOnce performs a single scan of friends, detecting unknown accounts and alerting the admin.
func (w *FriendWatcher) CheckOnce(ctx context.Context) ([]UnknownFriendDetection, error) {
	if w.client == nil {
		return nil, fmt.Errorf("kakao client is not configured")
	}

	// 1. Fetch current friends list (support pagination if total count > 100)
	var allFriends []kakao.Friend
	seenUUIDs := make(map[string]bool)
	offset := 0
	limit := 100

	for {
		resp, err := w.client.Friends().GetFriends(ctx, kakao.FriendsQueryOptions{
			Offset: offset,
			Limit:  limit,
		})
		if err != nil {
			return nil, fmt.Errorf("get friends: %w", err)
		}

		for _, friend := range resp.Elements {
			if !seenUUIDs[friend.UUID] {
				seenUUIDs[friend.UUID] = true
				allFriends = append(allFriends, friend)
			}
		}

		if len(allFriends) >= resp.TotalCount || len(resp.Elements) == 0 || resp.AfterURL == "" {
			break
		}
		offset += len(resp.Elements)
	}

	// 2. Identify unknown friends not present in the directory
	var detections []UnknownFriendDetection

	w.mu.Lock()
	defer w.mu.Unlock()

	for _, friend := range allFriends {
		_, isRegistered := w.directory.FindByUUID(friend.UUID)
		if isRegistered {
			continue
		}

		// Friend is unregistered
		_, alreadyAlerted := w.state.Alerted[friend.UUID]
		detection := UnknownFriendDetection{
			Friend:     friend,
			IsNewAlert: !alreadyAlerted,
		}

		if !alreadyAlerted && w.alertOnUnknown {
			// Find admin recipient
			adminMapping, found := w.directory.FindByEmail(w.adminEmail)
			if !found {
				adminMapping, found = w.directory.GetAdmin()
			}

			if found && adminMapping != nil && adminMapping.UUID != "" {
				now := time.Now().Local()
				msgStatus := "불가"
				if friend.AllowedMsg {
					msgStatus = "가능"
				}

				alertMsg := fmt.Sprintf(
					"🚨 [보안 경고] 미등록 신규 카카오톡 친구 감지!\n\n"+
						"화이트리스트에 등록되지 않은 신규 계정이 카카오봇에 연결되었습니다:\n\n"+
						"• 닉네임: %s\n"+
						"• 카카오 ID: %d\n"+
						"• Receiver UUID: %s\n"+
						"• 메시지 발송: %s\n"+
						"• 감지 일시: %s\n\n"+
						"⚠️ 본인이 승인한 사용자가 아니라면 즉시 카카오 개발자 콘솔의 연결된 사용자 목록을 확인하고 차단하세요.",
					friend.ProfileNickname,
					friend.ID,
					friend.UUID,
					msgStatus,
					now.Format("2006-01-02 15:04:05 MST"),
				)

				// Send KakaoTalk message to admin
				sendRes, sendErr := w.client.Friends().SendText(ctx, []string{adminMapping.UUID}, kakao.TextMessageRequest{
					Text: alertMsg,
				})

				if sendErr == nil && sendRes != nil && len(sendRes.SuccessfulReceiverUUIDs) > 0 {
					detection.AlertSent = true
					w.state.Alerted[friend.UUID] = AlertRecord{
						UUID:      friend.UUID,
						Nickname:  friend.ProfileNickname,
						KakaoID:   friend.ID,
						AlertedAt: now,
					}
					_ = w.saveStateUnlocked()
				} else {
					detection.AlertError = sendErr
					if detection.AlertError == nil && sendRes != nil && len(sendRes.FailureInfo) > 0 {
						detection.AlertError = fmt.Errorf("delivery failure: %s", sendRes.FailureInfo[0].Msg)
					}
				}
			} else {
				detection.AlertError = fmt.Errorf("admin recipient (%s) not found in friends directory", w.adminEmail)
			}
		}

		detections = append(detections, detection)
	}

	return detections, nil
}

// Start launches the background periodic friend watcher.
func (w *FriendWatcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.isRunning {
		w.mu.Unlock()
		return nil
	}

	watchCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.done = make(chan struct{})
	w.isRunning = true
	w.mu.Unlock()

	go w.runLoop(watchCtx)
	return nil
}

func (w *FriendWatcher) runLoop(ctx context.Context) {
	defer close(w.done)

	// Immediate first check
	w.logf("[FriendWatcher] Performing initial scan for unauthorized friends...")
	if detections, err := w.CheckOnce(ctx); err != nil {
		w.logf("[FriendWatcher] Initial scan error: %v", err)
	} else if len(detections) > 0 {
		w.logf("[FriendWatcher] Warning: detected %d unauthorized friend(s)", len(detections))
	} else {
		w.logf("[FriendWatcher] Initial scan complete: all friends are authorized.")
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logf("[FriendWatcher] Stopping watcher loop.")
			return
		case <-ticker.C:
			if detections, err := w.CheckOnce(ctx); err != nil {
				w.logf("[FriendWatcher] Periodic scan error: %v", err)
			} else if len(detections) > 0 {
				newCount := 0
				for _, d := range detections {
					if d.IsNewAlert {
						newCount++
					}
				}
				if newCount > 0 {
					w.logf("[FriendWatcher] 🚨 Detected %d NEW unauthorized friend(s)! Alert sent to admin.", newCount)
				}
			}
		}
	}
}

// Stop terminates the background friend watcher.
func (w *FriendWatcher) Stop() {
	w.mu.Lock()
	if !w.isRunning {
		w.mu.Unlock()
		return
	}
	if w.cancel != nil {
		w.cancel()
	}
	w.isRunning = false
	w.mu.Unlock()

	<-w.done
}

func (w *FriendWatcher) logf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if w.out != nil {
		fmt.Fprintln(w.out, msg)
	} else {
		log.Println(msg)
	}
}
