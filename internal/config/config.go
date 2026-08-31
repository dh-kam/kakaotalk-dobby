package config

import (
	"bufio"
	"os"
	"strings"

	"github.com/dh-kam/kakao-bot/pkg/ai"
	"github.com/dh-kam/kakao-bot/pkg/kakao"
)

// AppConfig contains common application configuration.
type AppConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenPath    string

	AIProvider     string
	AIAPIKey       string
	AIBaseURL      string
	AIModel        string
	AISystemPrompt string
}

// Load loads configuration from .env file and environment variables with sensible defaults.
func Load() *AppConfig {
	loadDotEnv(".env")

	aiKey := os.Getenv("AI_API_KEY")
	if aiKey == "" {
		if k := os.Getenv("OPENAI_API_KEY"); k != "" {
			aiKey = k
		} else if k := os.Getenv("GEMINI_API_KEY"); k != "" {
			aiKey = k
		} else if k := os.Getenv("ANTHROPIC_API_KEY"); k != "" {
			aiKey = k
		}
	}

	aiProvider := os.Getenv("AI_PROVIDER")
	if aiProvider == "" {
		if os.Getenv("GEMINI_API_KEY") != "" && os.Getenv("OPENAI_API_KEY") == "" {
			aiProvider = "gemini"
		} else if os.Getenv("ANTHROPIC_API_KEY") != "" && os.Getenv("OPENAI_API_KEY") == "" {
			aiProvider = "claude"
		} else if aiKey != "" {
			aiProvider = "openai"
		} else {
			aiProvider = "mock"
		}
	}

	cfg := &AppConfig{
		ClientID:       os.Getenv("KAKAO_REST_API_KEY"),
		ClientSecret:   os.Getenv("KAKAO_CLIENT_SECRET"),
		RedirectURI:    os.Getenv("KAKAO_REDIRECT_URI"),
		TokenPath:      os.Getenv("KAKAO_TOKEN_PATH"),
		AIProvider:     aiProvider,
		AIAPIKey:       aiKey,
		AIBaseURL:      os.Getenv("AI_BASE_URL"),
		AIModel:        os.Getenv("AI_MODEL"),
		AISystemPrompt: os.Getenv("AI_SYSTEM_PROMPT"),
	}

	if cfg.RedirectURI == "" {
		cfg.RedirectURI = "http://localhost:8080/callback"
	}
	if cfg.TokenPath == "" {
		cfg.TokenPath = kakao.DefaultTokenPath()
	}
	if cfg.AISystemPrompt == "" {
		cfg.AISystemPrompt = "You are a helpful and intelligent AI assistant for the KakaoTalk channel @0xc0de1ab. Keep your responses concise, friendly, and formatted nicely for mobile chat screens."
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

// BuildAIProvider constructs an ai.Provider interface from configuration.
func (c *AppConfig) BuildAIProvider() (ai.Provider, error) {
	return ai.NewProvider(ai.ProviderConfig{
		ProviderName: c.AIProvider,
		APIKey:       c.AIAPIKey,
		BaseURL:      c.AIBaseURL,
		Model:        c.AIModel,
	})
}
