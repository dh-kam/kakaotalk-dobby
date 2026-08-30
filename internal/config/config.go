package config

import (
	"bufio"
	"os"
	"strings"

	"github.com/dh-kam/kakao-bot/pkg/kakao"
)

// AppConfig contains common application configuration.
type AppConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenPath    string
}

// Load loads configuration from .env file and environment variables with sensible defaults.
func Load() *AppConfig {
	loadDotEnv(".env")

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

func loadDotEnv(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, val)
			}
		}
	}
}

// BuildKakaoClient constructs a kakao.Client interface from configuration.
func (c *AppConfig) BuildKakaoClient() kakao.Client {
	tokenStore := kakao.NewFileTokenStore(c.TokenPath)

	return kakao.NewClient(kakao.ClientConfig{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURI:  c.RedirectURI,
		TokenStore:   tokenStore,
	})
}
