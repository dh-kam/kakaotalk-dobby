package agent

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dh-kam/kakaotalk-dobby/pkg/academy"
	"github.com/dh-kam/kakaotalk-dobby/pkg/agent"
	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
)

// AgentRunRequest holds parameters for executing an Agent prompt.
type AgentRunRequest struct {
	ProviderName string
	Model        string
	Project      string
	Location     string
	Region       string
	BearerToken  string
	APIKey       string
	AccessKeyID  string
	SecretKey    string
	Prompt       string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenPath    string
	Out          io.Writer
}

// AgentRunUseCase runs the Agent with pluggable Vertex AI or Bedrock models.
type AgentRunUseCase struct{}

func NewAgentRunUseCase() *AgentRunUseCase {
	return &AgentRunUseCase{}
}

func (uc *AgentRunUseCase) Execute(ctx context.Context, req AgentRunRequest) error {
	out := req.Out
	if out == nil {
		out = os.Stdout
	}

	if req.Prompt == "" {
		return fmt.Errorf("prompt is required")
	}

	llmProvider, err := agent.NewLLMProvider(agent.ProviderOptions{
		ProviderName: req.ProviderName,
		Model:        req.Model,
		Project:      req.Project,
		Location:     req.Location,
		Region:       req.Region,
		BearerToken:  req.BearerToken,
		APIKey:       req.APIKey,
		AccessKeyID:  req.AccessKeyID,
		SecretKey:    req.SecretKey,
	})
	if err != nil {
		return fmt.Errorf("init llm provider: %w", err)
	}

	// Setup tools
	registry := agent.NewToolRegistry()
	registry.Register(&agent.ServerStatusTool{})

	busSvc := academy.NewService()
	_ = busSvc.LoadFromDir("data/schedules")
	_ = busSvc.LoadFromDir("data")
	registry.Register(agent.NewBusScheduleTool(busSvc))

	if req.ClientID != "" {
		kakaoClient := kakao.NewClient(kakao.ClientConfig{
			ClientID:     req.ClientID,
			ClientSecret: req.ClientSecret,
			RedirectURI:  req.RedirectURI,
			TokenStore:   kakao.NewFileTokenStore(req.TokenPath),
		})
		registry.Register(agent.NewSendKakaoMessageTool(kakaoClient))
	}

	botAgent := agent.NewAgent(agent.AgentConfig{
		Provider: llmProvider,
		Tools:    registry,
		SystemPrompt: `You are an intelligent KakaoBot Agent. You have access to tools for looking up academy shuttle bus schedules (e.g. 정상어학원, 강의하는아이들), checking server metrics, and sending KakaoTalk messages.
Think step-by-step and call appropriate tools when needed. Always respond in clear, polite Korean.

Domain Knowledge:
- "우미린 2차" (or "우미린2차") is officially also known as "우미린더스카이" or "더스카이" (구미 산동 확장단지 우미린 센트럴파크/더스카이). When users ask about "우미린더스카이" or "더스카이", search for "우미린2차" bus stops.
- "우미린 1차" is known as "우미린 풀하우스".
- "정상어학원" shuttle routes serve both "산동옥계 정상어학원" and "강의하는아이들".`,
		MaxIterations: 5,
	})

	fmt.Fprintf(out, "🤖 Running Agent with [%s] provider...\n\n", llmProvider.Name())
	result, err := botAgent.Run(ctx, req.Prompt)
	if err != nil {
		return fmt.Errorf("agent run failed: %w", err)
	}

	for _, step := range result.Steps {
		if step.Thought != "" {
			fmt.Fprintf(out, "💭 [Step %d Thought]\n%s\n\n", step.Iteration, step.Thought)
		}
		for _, tc := range step.ToolCalls {
			fmt.Fprintf(out, "🔧 [Action Tool Call]: %s(args: %s)\n", tc.Name, tc.Arguments)
		}
		for _, tr := range step.ToolResults {
			fmt.Fprintf(out, "📋 [Tool Observation]: %s\n\n", tr.Output)
		}
	}

	fmt.Fprintf(out, "🎯 [Final Answer]\n%s\n\n", result.Output)
	fmt.Fprintf(out, "📊 Total Tokens: %d (Input: %d, Output: %d)\n",
		result.Usage.TotalTokens, result.Usage.InputTokens, result.Usage.OutputTokens)

	return nil
}
