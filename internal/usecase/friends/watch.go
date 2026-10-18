package friends

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
)

// WatchFriendsRequest holds options for running the periodic friend watcher.
type WatchFriendsRequest struct {
	ClientID       string
	ClientSecret   string
	RedirectURI    string
	TokenPath      string
	FriendsMapPath string
	StatePath      string
	AdminEmail     string
	Interval       time.Duration
	Alert          bool
	Out            io.Writer
}

// WatchFriendsUseCase runs the continuous friend watcher until context cancellation or interrupt signal.
type WatchFriendsUseCase struct{}

// NewWatchFriendsUseCase constructs a WatchFriendsUseCase.
func NewWatchFriendsUseCase() *WatchFriendsUseCase {
	return &WatchFriendsUseCase{}
}

// Execute starts the watcher loop and blocks until interrupt or context cancellation.
func (uc *WatchFriendsUseCase) Execute(ctx context.Context, req WatchFriendsRequest) error {
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
		Interval:       req.Interval,
		AlertOnUnknown: req.Alert,
		Out:            out,
	})

	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(out, "👀 Watching for unauthorized KakaoTalk friends (Interval: %s, Admin: %s)...\n", req.Interval, req.AdminEmail)
	if err := watcher.Start(sigCtx); err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}

	<-sigCtx.Done()
	fmt.Fprintln(out, "\nShutting down friend watcher...")
	watcher.Stop()
	return nil
}
