package config

import (
	"bufio"
	"os"
	"strings"

	"github.com/dh-kam/kakaotalk-dobby/pkg/ai"
	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
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

	VertexAPIKey       string
	VertexProject      string
	VertexLocation     string
	VertexModel        string
	BedrockBearerToken string
	BedrockRegion      string
	DataDir            string
	AdminToken         string

	GroqAPIKey            string
	STTFallbackLocal       bool
	LocalWhisperModelPath  string
	LocalWhisperBinPath    string
}

// resolveAIKey picks the API key matching the requested provider first, so an
// explicit AI_PROVIDER never gets another provider's key, then falls back to
// the generic precedence chain for auto-detection.
func resolveAIKey(explicitProvider string) string {
	if k := os.Getenv("AI_API_KEY"); k != "" {
		return k
	}

	var candidates []string
	switch strings.ToLower(strings.TrimSpace(explicitProvider)) {
	case "grok", "xai":
		candidates = []string{"XAI_API_KEY"}
	case "gemini", "google":
		candidates = []string{"GEMINI_API_KEY", "VERTEX_API_KEY"}
	case "claude", "anthropic":
		candidates = []string{"ANTHROPIC_API_KEY"}
	case "openai":
		candidates = []string{"OPENAI_API_KEY"}
	}
	for _, name := range candidates {
		if k := os.Getenv(name); k != "" {
			return k
		}
	}

	for _, name := range []string{"OPENAI_API_KEY", "GEMINI_API_KEY", "VERTEX_API_KEY", "ANTHROPIC_API_KEY", "XAI_API_KEY"} {
		if k := os.Getenv(name); k != "" {
			return k
		}
	}
	return ""
}

// Load loads configuration from .env file and environment variables with sensible defaults.
func Load() *AppConfig {
	loadDotEnv(".env")

	aiKey := resolveAIKey(os.Getenv("AI_PROVIDER"))

	aiProvider := os.Getenv("AI_PROVIDER")
	if aiProvider == "" {
		if os.Getenv("GEMINI_API_KEY") != "" || os.Getenv("VERTEX_API_KEY") != "" {
			aiProvider = "gemini"
		} else if os.Getenv("ANTHROPIC_API_KEY") != "" && os.Getenv("OPENAI_API_KEY") == "" {
			aiProvider = "claude"
		} else if os.Getenv("XAI_API_KEY") != "" && aiKey == os.Getenv("XAI_API_KEY") {
			aiProvider = "grok"
		} else if aiKey != "" {
			aiProvider = "openai"
		} else {
			aiProvider = "mock"
		}
	}

	vertexProject := os.Getenv("VERTEX_PROJECT")
	if vertexProject == "" {
		vertexProject = os.Getenv("GCP_PROJECT")
	}
	if vertexProject == "" {
		vertexProject = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}

	vertexLocation := os.Getenv("VERTEX_LOCATION")
	if vertexLocation == "" || vertexLocation == "global" {
		vertexLocation = "us-central1"
	}

	bedrockRegion := os.Getenv("AWS_REGION")
	if bedrockRegion == "" {
		bedrockRegion = os.Getenv("BEDROCK_REGION")
	}
	if bedrockRegion == "" {
		bedrockRegion = "us-east-1"
	}

	aiModel := os.Getenv("AI_MODEL")
	if aiModel == "" && (aiProvider == "gemini" || aiProvider == "google") {
		aiModel = "gemini-3.8-flash"
	}
	vertexModel := os.Getenv("VERTEX_MODEL")
	if vertexModel == "" {
		vertexModel = "gemini-3.8-flash"
	}

	cfg := &AppConfig{
		ClientID:           os.Getenv("KAKAO_REST_API_KEY"),
		ClientSecret:       os.Getenv("KAKAO_CLIENT_SECRET"),
		RedirectURI:        os.Getenv("KAKAO_REDIRECT_URI"),
		TokenPath:          os.Getenv("KAKAO_TOKEN_PATH"),
		AIProvider:         aiProvider,
		AIAPIKey:           aiKey,
		AIBaseURL:          os.Getenv("AI_BASE_URL"),
		AIModel:            aiModel,
		AISystemPrompt:     os.Getenv("AI_SYSTEM_PROMPT"),
		VertexAPIKey:       os.Getenv("VERTEX_API_KEY"),
		VertexProject:      vertexProject,
		VertexLocation:     vertexLocation,
		VertexModel:        vertexModel,
		BedrockBearerToken: os.Getenv("AWS_BEARER_TOKEN_BEDROCK"),
		BedrockRegion:      bedrockRegion,
		DataDir:            getDataDir(),
		AdminToken:         getAdminToken(),
		GroqAPIKey:            os.Getenv("GROQ_API_KEY"),
		STTFallbackLocal:       strings.EqualFold(os.Getenv("STT_ENABLE_FALLBACK"), "true") || os.Getenv("STT_ENABLE_FALLBACK") == "1",
		LocalWhisperModelPath:  os.Getenv("WHISPER_MODEL_PATH"),
		LocalWhisperBinPath:    os.Getenv("WHISPER_BIN_PATH"),
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
	apiKey := c.AIAPIKey
	if c.AIProvider == "gemini" && c.VertexAPIKey != "" {
		apiKey = c.VertexAPIKey
	}

	return ai.NewProvider(ai.ProviderConfig{
		ProviderName: c.AIProvider,
		APIKey:       apiKey,
		BaseURL:      c.AIBaseURL,
		Model:        c.AIModel,
	})
}

func getDataDir() string {
	if d := os.Getenv("KAKAO_DATA_DIR"); d != "" {
		return d
	}
	if d := os.Getenv("DATA_DIR"); d != "" {
		return d
	}
	return "data/schedules"
}

func getAdminToken() string {
	if t := os.Getenv("KAKAO_ADMIN_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("ADMIN_TOKEN")
}
