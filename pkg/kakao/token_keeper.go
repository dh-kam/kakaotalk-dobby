package kakao

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

// DefaultTokenKeeperInterval is the default periodic interval to check and refresh tokens (24 hours).
const DefaultTokenKeeperInterval = 24 * time.Hour

// TokenKeeperConfig holds configuration for the background token keeper.
type TokenKeeperConfig struct {
	// Client is the primary Kakao client to keep refreshed.
	Client Client

	// Registry optionally manages multiple Kakao bot clients.
	// If both Client and Registry are set, Registry entries plus Client (if not in Registry) will be refreshed.
	Registry ClientRegistry

	// Interval specifies how frequently tokens are refreshed. Defaults to 24h.
	Interval time.Duration

	// CheckOnStart triggers an immediate token refresh upon starting.
	CheckOnStart bool

	// Logger receives formatted log messages. Defaults to log.Printf.
	Logger func(format string, args ...any)
}

// TokenKeeper runs a background worker that periodically refreshes Kakao OAuth tokens.
type TokenKeeper struct {
	cfg     TokenKeeperConfig
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	running bool
}

// NewTokenKeeper creates a new background token keeper instance.
func NewTokenKeeper(cfg TokenKeeperConfig) *TokenKeeper {
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultTokenKeeperInterval
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Printf
	}
	return &TokenKeeper{
		cfg: cfg,
	}
}

// Start begins the background token refresh loop.
func (k *TokenKeeper) Start(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.running {
		return fmt.Errorf("token keeper is already running")
	}

	keeperCtx, cancel := context.WithCancel(ctx)
	k.cancel = cancel
	k.running = true

	k.wg.Add(1)
	go k.loop(keeperCtx)

	return nil
}

// Stop gracefully terminates the background token refresh loop.
func (k *TokenKeeper) Stop() {
	k.mu.Lock()
	if !k.running {
		k.mu.Unlock()
		return
	}
	k.cancel()
	k.running = false
	k.mu.Unlock()

	k.wg.Wait()
}

// RefreshNow triggers an immediate token refresh across all configured clients.
func (k *TokenKeeper) RefreshNow(ctx context.Context) error {
	clients := k.gatherClients()
	if len(clients) == 0 {
		return nil
	}

	var errs []error
	for name, client := range clients {
		if err := k.refreshClient(ctx, name, client); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (k *TokenKeeper) loop(ctx context.Context) {
	defer k.wg.Done()

	if k.cfg.CheckOnStart {
		if err := k.RefreshNow(ctx); err != nil {
			k.cfg.Logger("[TokenKeeper] initial refresh warning: %v", err)
		}
	}

	ticker := time.NewTicker(k.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			k.cfg.Logger("[TokenKeeper] stopped")
			return
		case <-ticker.C:
			k.cfg.Logger("[TokenKeeper] running periodic token refresh check...")
			if err := k.RefreshNow(ctx); err != nil {
				k.cfg.Logger("[TokenKeeper] periodic refresh warning: %v", err)
			}
		}
	}
}

func (k *TokenKeeper) refreshClient(ctx context.Context, name string, client Client) error {
	if client == nil {
		return nil
	}

	store := client.GetTokenStore()
	if store == nil {
		return fmt.Errorf("bot %q: token store is not configured", name)
	}

	token, err := store.Load(ctx)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			k.cfg.Logger("[TokenKeeper] bot %q: no token file found yet, skipping refresh", name)
			return nil
		}
		return fmt.Errorf("bot %q: load token: %w", name, err)
	}

	if token.RefreshToken == "" {
		k.cfg.Logger("[TokenKeeper] bot %q: refresh token is empty, skipping refresh", name)
		return nil
	}

	authSvc := client.Auth()
	if authSvc == nil {
		return fmt.Errorf("bot %q: auth service is not configured", name)
	}

	refreshed, err := authSvc.RefreshToken(ctx, token.RefreshToken)
	if err != nil {
		return fmt.Errorf("bot %q: kakao refresh token request failed: %w", name, err)
	}

	if err := store.Save(ctx, refreshed); err != nil {
		return fmt.Errorf("bot %q: save refreshed token failed: %w", name, err)
	}

	k.cfg.Logger("[TokenKeeper] bot %q: token successfully refreshed (access expires in %ds, refresh expires in %ds)",
		name, refreshed.ExpiresIn, refreshed.RefreshTokenExpiresIn)
	return nil
}

func (k *TokenKeeper) gatherClients() map[string]Client {
	result := make(map[string]Client)

	if k.cfg.Registry != nil {
		for _, name := range k.cfg.Registry.List() {
			c, err := k.cfg.Registry.Get(name)
			if err == nil && c != nil {
				result[name] = c
			}
		}
	}

	if k.cfg.Client != nil {
		if _, exists := result["default"]; !exists {
			result["default"] = k.cfg.Client
		}
	}

	return result
}
