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

// AgentConfig configures the ReAct Agent.
type AgentConfig struct {
	Provider      LLMProvider
	Tools         *ToolRegistry
	SystemPrompt  string
	MaxIterations int
}

// NewAgent creates a new ReAct Agent.
func NewAgent(cfg AgentConfig) Agent {
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 5
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = "You are a helpful and reliable AI Assistant. Solve user queries by reasoning and calling appropriate tools step-by-step."
	}
	return &defaultAgent{
		provider:      cfg.Provider,
		tools:         cfg.Tools,
		systemPrompt:  cfg.SystemPrompt,
		maxIterations: cfg.MaxIterations,
	}
}

func (a *defaultAgent) GetTools() []Tool {
	return a.tools.List()
}

func (a *defaultAgent) GetProvider() LLMProvider {
	return a.provider
}

// Run executes the ReAct reasoning & tool execution loop until completion.
func (a *defaultAgent) Run(ctx context.Context, input string) (*AgentResult, error) {
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
			if len(steps) > 0 && len(steps[len(steps)-1].ToolResults) > 0 {
				// If a tool was executed in previous step, return tool result safely
				lastToolRes := steps[len(steps)-1].ToolResults[0]
				return &AgentResult{
					Output: lastToolRes.Output,
					Steps:  steps,
					Usage:  totalUsage,
				}, nil
			}
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
	if len(lastStep.ToolResults) > 0 {
		return &AgentResult{
			Output: lastStep.ToolResults[0].Output,
			Steps:  steps,
			Usage:  totalUsage,
		}, nil
	}

	return &AgentResult{
		Output: fmt.Sprintf("%s\n(Reached maximum reasoning iterations: %d)", lastStep.Thought, a.maxIterations),
		Steps:  steps,
		Usage:  totalUsage,
	}, nil
}
