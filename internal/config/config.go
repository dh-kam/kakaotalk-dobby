package config

import (
	"os"

	"github.com/dh-kam/kakao-bot/pkg/kakao"
)

// AppConfig contains common application configuration.
type AppConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenPath    string
}

// Load loads configuration from environment variables with sensible defaults.
func Load() *AppConfig {
	cfg := &AppConfig{
		ClientID:     os.Getenv("KAKAO_REST_API_KEY"),
		ClientSecret: os.Getenv("KAKAO_CLIENT_SECRET"),
		RedirectURI:  os.Getenv("KAKAO_REDIRECT_URI"),
		TokenPath:    os.Getenv("KAKAO_TOKEN_PATH"),
	}

	if cfg.RedirectURI == "" {
		cfg.RedirectURI = "http://localhost:8080/callback"
	}
	if cfg.TokenPath == "" {
		cfg.TokenPath = kakao.DefaultTokenPath()
	}

	return cfg
}

// BuildKakaoClient constructs a kakao.Client from configuration.
func (c *AppConfig) BuildKakaoClient() *kakao.Client {
	oauthClient := kakao.NewOAuthClient(kakao.OAuthConfig{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURI:  c.RedirectURI,
	})

	tokenStore := kakao.NewFileTokenStore(c.TokenPath)

	return kakao.NewClient(kakao.ClientConfig{
		OAuthClient: oauthClient,
		TokenStore:  tokenStore,
	})
}
