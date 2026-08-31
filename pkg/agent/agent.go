package agent

import (
	"context"
	"fmt"
	"strings"
)

type defaultAgent struct {
	provider      LLMProvider
	tools         *ToolRegistry
	systemPrompt  string
	maxIterations int
}

// AgentConfig holds settings to configure an Agent.
type AgentConfig struct {
	Provider      LLMProvider
	Tools         *ToolRegistry
	SystemPrompt  string
	MaxIterations int
}

// NewAgent creates a new Agent instance.
func NewAgent(cfg AgentConfig) Agent {
	maxIter := cfg.MaxIterations
	if maxIter <= 0 {
		maxIter = 5
	}
	sysPrompt := cfg.SystemPrompt
	if sysPrompt == "" {
		sysPrompt = "You are a smart, autonomous AI assistant. You can reason step-by-step and call available tools when needed to answer user questions."
	}
	tools := cfg.Tools
	if tools == nil {
		tools = NewToolRegistry()
	}
	return &defaultAgent{
		provider:      cfg.Provider,
		tools:         tools,
		systemPrompt:  sysPrompt,
		maxIterations: maxIter,
	}
}

func (a *defaultAgent) GetProvider() LLMProvider {
	return a.provider
}

func (a *defaultAgent) GetTools() []Tool {
	return a.tools.List()
}

func (a *defaultAgent) Run(ctx context.Context, input string) (*AgentResult, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("llm provider is not configured")
	}

	messages := []Message{
		{Role: "user", Content: input},
	}

	toolDefs := a.tools.GetDefinitions()
	var steps []AgentStep
	var totalUsage TokenUsage

	for iteration := 1; iteration <= a.maxIterations; iteration++ {
		req := ToolCompletionRequest{
			CompletionRequest: CompletionRequest{
				SystemPrompt: a.systemPrompt,
				Messages:     messages,
			},
			Tools: toolDefs,
		}

		resp, err := a.provider.GenerateWithTools(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("llm generate error at step %d: %w", iteration, err)
		}

		totalUsage.InputTokens += resp.Usage.InputTokens
		totalUsage.OutputTokens += resp.Usage.OutputTokens
		totalUsage.TotalTokens += resp.Usage.TotalTokens

		step := AgentStep{
			Iteration: iteration,
			Thought:   resp.Text,
			ToolCalls: resp.ToolCalls,
		}

		// If no tools were called, the model provided its final answer
		if len(resp.ToolCalls) == 0 {
			steps = append(steps, step)
			return &AgentResult{
				Output: strings.TrimSpace(resp.Text),
				Steps:  steps,
				Usage:  totalUsage,
			}, nil
		}

		// Record assistant message with tool calls
		messages = append(messages, Message{
			Role:      "assistant",
			Content:   resp.Text,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each requested tool call
		for _, tc := range resp.ToolCalls {
			tool, exists := a.tools.Get(tc.Name)
			var toolOutput string
			var toolErr error

			if !exists {
				toolErr = fmt.Errorf("tool %q not found", tc.Name)
				toolOutput = fmt.Sprintf("Error: tool %q is not available", tc.Name)
			} else {
				toolOutput, toolErr = tool.Execute(ctx, tc.Arguments)
				if toolErr != nil {
					toolOutput = fmt.Sprintf("Error executing tool %s: %v", tc.Name, toolErr)
				}
			}

			toolRes := ToolResult{
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
				Output:     toolOutput,
				Error:      toolErr,
			}
			step.ToolResults = append(step.ToolResults, toolRes)

			// Append tool observation message
			messages = append(messages, Message{
				Role:       "tool",
				Content:    toolOutput,
				ToolCallID: tc.ID,
				Name:       tc.Name,
			})
		}

		steps = append(steps, step)
	}

	lastStep := steps[len(steps)-1]
	return &AgentResult{
		Output: fmt.Sprintf("%s\n(Reached maximum reasoning iterations: %d)", lastStep.Thought, a.maxIterations),
		Steps:  steps,
		Usage:  totalUsage,
	}, nil
}
