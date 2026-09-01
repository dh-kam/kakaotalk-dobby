package main

import (
	"context"
	"fmt"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/internal/config"
	"github.com/dh-kam/kakaotalk-dobby/pkg/academy"
	"github.com/dh-kam/kakaotalk-dobby/pkg/agent"
)

func main() {
	cfg := config.Load()
	vProvider, err := agent.NewLLMProvider(agent.ProviderOptions{
		ProviderName: "vertex",
		Model:        "gemini-2.5-flash",
		Project:      cfg.VertexProject,
		Location:     cfg.VertexLocation,
		APIKey:       cfg.VertexAPIKey,
	})
	if err != nil {
		fmt.Printf("Init provider err: %v\n", err)
		return
	}

	busSvc := academy.NewService()
	_ = busSvc.LoadFromDir("data/schedules")

	reg := agent.NewToolRegistry()
	reg.Register(agent.NewBusScheduleTool(busSvc))

	botAgent := agent.NewAgent(agent.AgentConfig{
		Provider:      vProvider,
		Tools:         reg,
		SystemPrompt:  "You are a helpful assistant. Always answer nicely in Korean.",
		MaxIterations: 3,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := botAgent.Run(ctx, "정상어학원 2호차 버스 시간표 알려줘")
	if err != nil {
		fmt.Printf("Agent Run Error: %v\n", err)
		return
	}

	fmt.Printf("Steps: %d\n", len(res.Steps))
	for i, st := range res.Steps {
		fmt.Printf("Step %d: Thought=%q, ToolCalls=%+v, ToolResultsCount=%d\n", i+1, st.Thought, st.ToolCalls, len(st.ToolResults))
	}
	fmt.Printf("Final Output:\n%s\n", res.Output)
}
