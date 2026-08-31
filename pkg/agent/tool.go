package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"

	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
	"github.com/samber/lo"
)

// ToolRegistry manages registered tools.
type ToolRegistry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewToolRegistry creates an empty ToolRegistry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

func (r *ToolRegistry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

func (r *ToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *ToolRegistry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return lo.Values(r.tools)
}

func (r *ToolRegistry) GetDefinitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return lo.Map(lo.Values(r.tools), func(t Tool, _ int) ToolDefinition {
		return ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.ParametersSchema(),
		}
	})
}

// ServerStatusTool reports server health and system metrics.
type ServerStatusTool struct{}

func (t *ServerStatusTool) Name() string {
	return "get_server_status"
}

func (t *ServerStatusTool) Description() string {
	return "Retrieve current server system metrics including OS, Go runtime version, memory usage, and goroutine count."
}

func (t *ServerStatusTool) ParametersSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *ServerStatusTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	res := map[string]interface{}{
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
		"go_version":  runtime.Version(),
		"goroutines":  runtime.NumGoroutine(),
		"alloc_mb":    fmt.Sprintf("%.2f MB", float64(m.Alloc)/(1024*1024)),
		"total_alloc": fmt.Sprintf("%.2f MB", float64(m.TotalAlloc)/(1024*1024)),
	}
	bytes, _ := json.Marshal(res)
	return string(bytes), nil
}

// SendKakaoMessageTool allows the Agent to autonomously send KakaoTalk messages.
type SendKakaoMessageTool struct {
	client kakao.Client
}

func NewSendKakaoMessageTool(client kakao.Client) *SendKakaoMessageTool {
	return &SendKakaoMessageTool{client: client}
}

func (t *SendKakaoMessageTool) Name() string {
	return "send_kakao_message"
}

func (t *SendKakaoMessageTool) Description() string {
	return "Send a KakaoTalk notification message directly to the user."
}

func (t *SendKakaoMessageTool) ParametersSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "The message body text to send to KakaoTalk.",
			},
			"url": map[string]interface{}{
				"type":        "string",
				"description": "Optional web URL link attached to the message.",
			},
			"button_title": map[string]interface{}{
				"type":        "string",
				"description": "Optional button text label.",
			},
		},
		"required": []string{"message"},
	}
}

func (t *SendKakaoMessageTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	if t.client == nil {
		return "", fmt.Errorf("kakao client is not configured")
	}

	var args struct {
		Message     string `json:"message"`
		URL         string `json:"url"`
		ButtonTitle string `json:"button_title"`
	}

	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Message == "" {
		return "", fmt.Errorf("message field is required")
	}

	err := t.client.Memo().SendText(ctx, kakao.TextMessageRequest{
		Text:        args.Message,
		WebURL:      args.URL,
		ButtonTitle: args.ButtonTitle,
	})
	if err != nil {
		return "", fmt.Errorf("send kakao memo: %w", err)
	}

	return fmt.Sprintf("Successfully sent KakaoTalk message: %q", args.Message), nil
}
