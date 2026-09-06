package config

import (
	"bufio"
	"os"
	"strings"

	"github.com/dh-kam/kakaotalk-dobby/pkg/ai"
	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
	"github.com/samber/lo"
)

// BotConfig holds configuration for a specific Kakao bot application.
type BotConfig struct {
	Name         string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenPath    string
	IsDefault    bool
}

// AppConfig contains common application configuration.
type AppConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenPath    string

	DefaultBot string
	Bots       map[string]BotConfig

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

	defaultBot := strings.ToLower(strings.TrimSpace(os.Getenv("KAKAO_DEFAULT_BOT")))
	if defaultBot == "" {
		defaultBot = "dobby"
	}
	cfg.DefaultBot = defaultBot
	cfg.Bots = loadBots(cfg.ClientID, cfg.ClientSecret, cfg.RedirectURI, cfg.TokenPath, defaultBot)

	return cfg
}

func loadBots(baseClientID, baseSecret, baseRedirect, baseTokenPath, defaultBot string) map[string]BotConfig {
	bots := make(map[string]BotConfig)
	rawBots := os.Getenv("KAKAO_BOTS")

	if rawBots != "" {
		names := lo.Filter(strings.Split(rawBots, ","), func(s string, _ int) bool {
			return strings.TrimSpace(s) != ""
		})

		for _, rawName := range names {
			name := strings.ToLower(strings.TrimSpace(rawName))
			nameUpper := strings.ToUpper(strings.ReplaceAll(name, "-", "_"))

			clientID := os.Getenv("KAKAO_BOT_" + nameUpper + "_REST_API_KEY")
			if clientID == "" {
				clientID = os.Getenv("KAKAO_" + nameUpper + "_REST_API_KEY")
			}
			if clientID == "" && (name == defaultBot || name == "default") {
				clientID = baseClientID
			}

			clientSecret := os.Getenv("KAKAO_BOT_" + nameUpper + "_CLIENT_SECRET")
			if clientSecret == "" {
				clientSecret = os.Getenv("KAKAO_" + nameUpper + "_CLIENT_SECRET")
			}
			if clientSecret == "" && (name == defaultBot || name == "default") {
				clientSecret = baseSecret
			}

			redirectURI := os.Getenv("KAKAO_BOT_" + nameUpper + "_REDIRECT_URI")
			if redirectURI == "" {
				redirectURI = os.Getenv("KAKAO_" + nameUpper + "_REDIRECT_URI")
			}
			if redirectURI == "" {
				redirectURI = baseRedirect
			}

			tokenPath := os.Getenv("KAKAO_BOT_" + nameUpper + "_TOKEN_PATH")
			if tokenPath == "" {
				tokenPath = os.Getenv("KAKAO_" + nameUpper + "_TOKEN_PATH")
			}
			if tokenPath == "" && (name == defaultBot || name == "default") && baseTokenPath != "" {
				tokenPath = baseTokenPath
			}
			if tokenPath == "" {
				tokenPath = kakao.BotTokenPath(name, "")
			}

			isDef := name == defaultBot || (len(bots) == 0 && defaultBot == "")

			bots[name] = BotConfig{
				Name:         name,
				ClientID:     clientID,
				ClientSecret: clientSecret,
				RedirectURI:  redirectURI,
				TokenPath:    tokenPath,
				IsDefault:    isDef,
			}
		}
	}

	if len(bots) == 0 && baseClientID != "" {
		botName := defaultBot
		if botName == "" {
			botName = "default"
		}
		bots[botName] = BotConfig{
			Name:         botName,
			ClientID:     baseClientID,
			ClientSecret: baseSecret,
			RedirectURI:  baseRedirect,
			TokenPath:    baseTokenPath,
			IsDefault:    true,
		}
	}

	return bots
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

// GetBotConfig retrieves configuration for a specific bot by name.
func (c *AppConfig) GetBotConfig(name string) (BotConfig, bool) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" || normalized == "default" {
		if c.DefaultBot != "" {
			if cfg, ok := c.Bots[c.DefaultBot]; ok {
				return cfg, true
			}
		}
		for _, b := range c.Bots {
			if b.IsDefault {
				return b, true
			}
		}
		if len(c.Bots) > 0 {
			for _, b := range c.Bots {
				return b, true
			}
		}
		return BotConfig{
			Name:         "default",
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			RedirectURI:  c.RedirectURI,
			TokenPath:    c.TokenPath,
			IsDefault:    true,
		}, true
	}

	cfg, ok := c.Bots[normalized]
	return cfg, ok
}

// BuildKakaoRegistry constructs a kakao.ClientRegistry from configuration.
func (c *AppConfig) BuildKakaoRegistry() (kakao.ClientRegistry, error) {
	registry := kakao.NewClientRegistry()

	for name, botCfg := range c.Bots {
		tokenStore := kakao.NewFileTokenStore(botCfg.TokenPath)
		client := kakao.NewClient(kakao.ClientConfig{
			ClientID:     botCfg.ClientID,
			ClientSecret: botCfg.ClientSecret,
			RedirectURI:  botCfg.RedirectURI,
			TokenStore:   tokenStore,
		})
		registry.Register(name, client, botCfg.IsDefault)
	}

	if registry.Count() == 0 && c.ClientID != "" {
		tokenStore := kakao.NewFileTokenStore(c.TokenPath)
		client := kakao.NewClient(kakao.ClientConfig{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			RedirectURI:  c.RedirectURI,
			TokenStore:   tokenStore,
		})
		registry.Register("default", client, true)
	}

	return registry, nil
}

// BuildKakaoClient constructs the primary kakao.Client interface from configuration.
func (c *AppConfig) BuildKakaoClient() kakao.Client {
	reg, err := c.BuildKakaoRegistry()
	if err == nil && reg.Count() > 0 {
		if client, err := reg.GetDefault(); err == nil {
			return client
		}
	}

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
