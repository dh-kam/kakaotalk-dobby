package friends

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
)

// CheckFriendsRequest holds options for performing a one-time check for unauthorized friends.
type CheckFriendsRequest struct {
	ClientID       string
	ClientSecret   string
	RedirectURI    string
	TokenPath      string
	FriendsMapPath string
	StatePath      string
	AdminEmail     string
	Alert          bool
	Out            io.Writer
}

// CheckFriendsUseCase scans friends against the whitelist and alerts admin of unauthorized accounts.
type CheckFriendsUseCase struct{}

// NewCheckFriendsUseCase constructs a CheckFriendsUseCase.
func NewCheckFriendsUseCase() *CheckFriendsUseCase {
	return &CheckFriendsUseCase{}
}

// Execute performs the check and writes a structured report to Out.
func (uc *CheckFriendsUseCase) Execute(ctx context.Context, req CheckFriendsRequest) error {
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

	directory := NewFileFriendDirectory(req.FriendsMapPath)

	watcher := NewFriendWatcher(FriendWatcherConfig{
		Client:         client,
		Directory:      directory,
		StatePath:      req.StatePath,
		AdminEmail:     req.AdminEmail,
		AlertOnUnknown: req.Alert,
		Out:            out,
	})

	detections, err := watcher.CheckOnce(ctx)
	if err != nil {
		return fmt.Errorf("check friends: %w", err)
	}

	if len(detections) == 0 {
		fmt.Fprintf(out, "✅ All friends are verified in the whitelist (%d registered mappings).\n", len(directory.All()))
		return nil
	}

	fmt.Fprintf(out, "⚠️ Found %d unauthorized friend(s) not in whitelist:\n\n", len(detections))
	for i, d := range detections {
		status := "Allowed"
		if !d.Friend.AllowedMsg {
			status = "Blocked"
		}
		fmt.Fprintf(out, "  %d. %s (Kakao ID: %d, Message: %s)\n", i+1, d.Friend.ProfileNickname, d.Friend.ID, status)
		fmt.Fprintf(out, "     UUID: %s\n", d.Friend.UUID)

		if d.AlertSent {
			fmt.Fprintf(out, "     🚨 Security alert successfully sent to admin (%s) via KakaoTalk!\n", req.AdminEmail)
		} else if d.AlertError != nil {
			fmt.Fprintf(out, "     ❌ Failed to send alert: %v\n", d.AlertError)
		} else if !d.IsNewAlert {
			fmt.Fprintf(out, "     ℹ️ Alert already delivered previously (suppressed).\n")
		}
		fmt.Fprintln(out)
	}

	return nil
}
