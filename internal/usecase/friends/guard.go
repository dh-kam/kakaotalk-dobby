package friends

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/internal/config"
	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
)

// ErrUnverifiedFriendBlocked is returned when sending a message is blocked because
// recipients have unverified identities (e.g. no verified email address or unregistered in whitelist).
var ErrUnverifiedFriendBlocked = errors.New("message blocked: recipient identity unverified")

// UnverifiedRecipientInfo contains details about a recipient that failed verification.
type UnverifiedRecipientInfo struct {
	UUID       string
	Name       string
	Registered bool
	Reason     string
}

// BlockedRecipientError provides structured details on which recipients were blocked.
type BlockedRecipientError struct {
	Unverified []UnverifiedRecipientInfo
}

func (e *BlockedRecipientError) Error() string {
	details := make([]string, len(e.Unverified))
	for i, u := range e.Unverified {
		details[i] = fmt.Sprintf("%s (%s: %s)", u.Name, u.UUID, u.Reason)
	}
	return fmt.Sprintf("message blocked: %d unverified recipient(s) [%s]", len(e.Unverified), strings.Join(details, ", "))
}

// Is supports errors.Is matching against ErrUnverifiedFriendBlocked.
func (e *BlockedRecipientError) Is(target error) bool {
	return target == ErrUnverifiedFriendBlocked
}

// AlertSender sends security and administrative alert messages to an admin.
type AlertSender interface {
	SendAlert(ctx context.Context, adminUUID string, text string) error
}

// AlertSenderFunc allows using a simple function as an AlertSender.
type AlertSenderFunc func(ctx context.Context, adminUUID string, text string) error

// SendAlert invokes the underlying function.
func (f AlertSenderFunc) SendAlert(ctx context.Context, adminUUID string, text string) error {
	return f(ctx, adminUUID, text)
}

// KakaoAlertSender delivers security alerts via KakaoTalk Friends API with friend guard disabled.
type KakaoAlertSender struct {
	client kakao.Client
}

// NewKakaoAlertSender creates an AlertSender backed by a Kakao client.
func NewKakaoAlertSender(client kakao.Client) *KakaoAlertSender {
	return &KakaoAlertSender{client: client}
}

// SendAlert sends the alert text to the specified admin UUID.
func (s *KakaoAlertSender) SendAlert(ctx context.Context, adminUUID string, text string) error {
	if s.client == nil {
		return fmt.Errorf("kakao alert client is nil")
	}
	res, err := s.client.Friends().SendText(ctx, []string{adminUUID}, kakao.TextMessageRequest{
		Text: text,
	})
	if err != nil {
		return fmt.Errorf("send alert to admin: %w", err)
	}
	if res != nil && len(res.FailureInfo) > 0 {
		return fmt.Errorf("alert delivery failed: %s", res.FailureInfo[0].Msg)
	}
	return nil
}

// DirectoryFriendGuardConfig holds configuration for DirectoryFriendGuard.
type DirectoryFriendGuardConfig struct {
	Directory   FriendDirectory
	AdminEmail  string                                                     // Admin email (default: "inzero@hanmail.net")
	AdminUUID   string                                                     // Fallback admin UUID if lookup fails
	AlertSender AlertSender                                                // Optional custom alert sender
	OnAlertSent func(unverified []UnverifiedRecipientInfo, summary string) // Optional callback for tests/telemetry
}

// DirectoryFriendGuard enforces friend identity verification before sending messages.
// Unverified friends (unregistered or missing verified email) are blocked, and an alert
// is dispatched to the admin prior to blocking.
type DirectoryFriendGuard struct {
	directory   FriendDirectory
	adminEmail  string
	adminUUID   string
	alertSender AlertSender
	onAlertSent func(unverified []UnverifiedRecipientInfo, summary string)
	mu          sync.Mutex
}

// NewDirectoryFriendGuard creates a new DirectoryFriendGuard.
func NewDirectoryFriendGuard(cfg DirectoryFriendGuardConfig) *DirectoryFriendGuard {
	dir := cfg.Directory
	if dir == nil {
		dir = NewFileFriendDirectory("")
	}
	adminEmail := strings.TrimSpace(cfg.AdminEmail)
	if adminEmail == "" {
		adminEmail = "inzero@hanmail.net"
	}
	adminUUID := strings.TrimSpace(cfg.AdminUUID)
	if adminUUID == "" {
		adminUUID = "MAQyBjICMQE5FSQUJB0sFCMWIQ08CToONwAzdA"
	}

	return &DirectoryFriendGuard{
		directory:   dir,
		adminEmail:  adminEmail,
		adminUUID:   adminUUID,
		alertSender: cfg.AlertSender,
		onAlertSent: cfg.OnAlertSent,
	}
}

// ValidateSend validates receiver UUIDs against the friend directory.
// Any recipient lacking a verified email is classified as unverified and blocked.
// If unverified recipients are detected, an alert is sent to the admin before returning.
// Valid recipients are returned in allowedUUIDs; if all recipients are unverified, ErrUnverifiedFriendBlocked is returned.
func (g *DirectoryFriendGuard) ValidateSend(ctx context.Context, receiverUUIDs []string, messageSummary string) ([]string, error) {
	if len(receiverUUIDs) == 0 {
		return receiverUUIDs, nil
	}

	var allowed []string
	var unverified []UnverifiedRecipientInfo

	seen := make(map[string]bool)

	for _, rawUUID := range receiverUUIDs {
		uuid := strings.TrimSpace(rawUUID)
		if uuid == "" || seen[uuid] {
			continue
		}
		seen[uuid] = true

		mapping, found := g.directory.FindByUUID(uuid)
		if !found {
			unverified = append(unverified, UnverifiedRecipientInfo{
				UUID:       uuid,
				Name:       "미등록 계정",
				Registered: false,
				Reason:     "화이트리스트(friends_map.json) 미등록 계정",
			})
			continue
		}

		if strings.TrimSpace(mapping.Email) == "" {
			name := mapping.Name
			if name == "" {
				name = "이름 미지정"
			}
			unverified = append(unverified, UnverifiedRecipientInfo{
				UUID:       uuid,
				Name:       name,
				Registered: true,
				Reason:     "이메일 미등록 (신원확인 미완료)",
			})
			continue
		}

		// Recipient is registered and has a verified email
		allowed = append(allowed, uuid)
	}

	if len(unverified) > 0 {
		// 관리자 선(先) 경고 발송 (Alert Before Block)
		g.alertAdmin(ctx, unverified, messageSummary)

		if len(allowed) == 0 {
			return nil, &BlockedRecipientError{Unverified: unverified}
		}
	}

	return allowed, nil
}

func (g *DirectoryFriendGuard) alertAdmin(ctx context.Context, unverified []UnverifiedRecipientInfo, messageSummary string) {
	adminUUID := g.resolveAdminUUID()
	if adminUUID == "" {
		log.Printf("[DirectoryFriendGuard] WARN: cannot send security alert, admin UUID not found")
		return
	}

	now := time.Now().Local()
	alertText := formatAdminAlertMessage(unverified, messageSummary, now)

	sender := g.getAlertSender()
	if sender != nil {
		if err := sender.SendAlert(ctx, adminUUID, alertText); err != nil {
			log.Printf("[DirectoryFriendGuard] ERROR: failed to dispatch admin alert: %v", err)
		}
	}

	if g.onAlertSent != nil {
		g.onAlertSent(unverified, messageSummary)
	}
}

func (g *DirectoryFriendGuard) resolveAdminUUID() string {
	if g.directory != nil {
		if admin, ok := g.directory.FindByEmail(g.adminEmail); ok && admin.UUID != "" {
			return admin.UUID
		}
		if admin, ok := g.directory.GetAdmin(); ok && admin.UUID != "" {
			return admin.UUID
		}
	}
	return g.adminUUID
}

func (g *DirectoryFriendGuard) getAlertSender() AlertSender {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.alertSender != nil {
		return g.alertSender
	}

	appCfg := config.Load()
	tokenStore := kakao.NewFileTokenStore(appCfg.TokenPath)
	alertClient := kakao.NewClient(kakao.ClientConfig{
		ClientID:           appCfg.ClientID,
		ClientSecret:       appCfg.ClientSecret,
		RedirectURI:        appCfg.RedirectURI,
		TokenStore:         tokenStore,
		DisableFriendGuard: true, // Prevents infinite recursion when sending alerts
	})

	g.alertSender = NewKakaoAlertSender(alertClient)
	return g.alertSender
}

func formatAdminAlertMessage(unverified []UnverifiedRecipientInfo, messageSummary string, now time.Time) string {
	var sb strings.Builder
	sb.WriteString("🚨 [보안 경고] 신원 미확인 카카오톡 친구 메시지 발송 차단!\n\n")
	sb.WriteString("신원 확인(이메일 등록)이 완료되지 않은 친구에게 메시지 발송 시도가 발생하여 최끝단에서 전송을 차단했습니다:\n\n")

	for i, u := range unverified {
		sb.WriteString(fmt.Sprintf("%d. 대상: %s\n", i+1, u.Name))
		sb.WriteString(fmt.Sprintf("   • Receiver UUID: %s\n", u.UUID))
		sb.WriteString(fmt.Sprintf("   • 차단 사유: %s\n", u.Reason))
	}

	sb.WriteString(fmt.Sprintf("\n• 차단 일시: %s\n", now.Format("2006-01-02 15:04:05 MST")))

	trimmedSummary := strings.TrimSpace(messageSummary)
	if trimmedSummary != "" {
		runes := []rune(trimmedSummary)
		if len(runes) > 200 {
			trimmedSummary = string(runes[:200]) + "..."
		}
		sb.WriteString(fmt.Sprintf("• 시도된 메시지 내용:\n  \"%s\"\n\n", trimmedSummary))
	} else {
		sb.WriteString("\n")
	}

	sb.WriteString("⚠️ 의도된 수신자라면 friends_map.json 에 해당 사용자의 이메일을 등록하여 신원을 인증해주세요.")
	return sb.String()
}
