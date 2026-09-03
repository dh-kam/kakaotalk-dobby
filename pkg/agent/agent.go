package agent

import (
	"context"
	"encoding/json"
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

// Run executes the ReAct reasoning loop without previous history.
func (a *defaultAgent) Run(ctx context.Context, input string) (*AgentResult, error) {
	return a.RunWithHistory(ctx, input, nil)
}

// RunWithHistory executes the ReAct reasoning loop preserving multi-turn conversation history.
func (a *defaultAgent) RunWithHistory(ctx context.Context, input string, history []Message) (*AgentResult, error) {
	var messages []Message
	if len(history) > 0 {
		messages = append(messages, history...)
	}
	messages = append(messages, Message{Role: "user", Content: input})

	toolDefs := a.tools.GetDefinitions()
	var steps []AgentStep
	var totalUsage TokenUsage

	dynamicPrompt := BuildDynamicSystemPrompt(a.systemPrompt)

	for iteration := 1; iteration <= a.maxIterations; iteration++ {
		req := ToolCompletionRequest{
			CompletionRequest: CompletionRequest{
				SystemPrompt: dynamicPrompt,
				Messages:     messages,
			},
			Tools: toolDefs,
		}

		resp, err := a.provider.GenerateWithTools(ctx, req)
		if err != nil {
			if len(steps) > 0 && len(steps[len(steps)-1].ToolResults) > 0 {
				// If a tool was executed in previous step, return tool result safely formatted
				lastToolRes := steps[len(steps)-1].ToolResults[0]
				return &AgentResult{
					Output: formatSafeToolOutput(lastToolRes.Output),
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

		stepToolFailures := 0

		// Execute each requested tool call (capped to max 5 calls to prevent flood)
		calls := resp.ToolCalls
		if len(calls) > 5 {
			calls = calls[:5]
		}

		for _, tc := range calls {
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

			if toolErr != nil {
				stepToolFailures++
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

		// Circuit Breaker: If all tools in this step failed, and previous step also had tool errors, abort early
		if stepToolFailures == len(calls) && len(steps) > 1 && steps[len(steps)-2].ToolResults != nil {
			prevFailed := true
			for _, tr := range steps[len(steps)-2].ToolResults {
				if tr.Error == nil {
					prevFailed = false
					break
				}
			}
			if prevFailed {
				return &AgentResult{
					Output: "요청하신 작업을 처리하는 중 반복적인 오류가 발생했습니다. 잠시 후 다시 시도해 주세요.",
					Steps:  steps,
					Usage:  totalUsage,
				}, nil
			}
		}
	}

	lastStep := steps[len(steps)-1]
	if len(lastStep.ToolResults) > 0 {
		return &AgentResult{
			Output: formatSafeToolOutput(lastStep.ToolResults[0].Output),
			Steps:  steps,
			Usage:  totalUsage,
		}, nil
	}

	out := strings.TrimSpace(lastStep.Thought)
	if out == "" {
		out = "요청하신 질문에 대한 정보 분석을 완료하지 못했습니다. 조금 더 구체적으로 말씀해 주시면 다시 안내해 드리겠습니다."
	}
	return &AgentResult{
		Output: out,
		Steps:  steps,
		Usage:  totalUsage,
	}, nil
}

// formatSafeToolOutput formats raw tool outputs (such as JSON arrays, objects, or error prefixes)
// into clean, user-facing Korean text, preventing raw developer payload leaks.
func formatSafeToolOutput(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return "조회 결과가 없습니다."
	}
	if strings.HasPrefix(trimmed, "Error:") || strings.HasPrefix(trimmed, "Error executing") {
		return "요청하신 정보를 조회하는 중 오류가 발생했습니다. 잠시 후 다시 시도해 주세요."
	}

	// 1. JSON Array formatting (e.g. bus timetable array)
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		var list []map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &list); err == nil && len(list) > 0 {
			var sb strings.Builder
			for _, item := range list {
				academy, _ := item["academy_name"].(string)
				vehicle, _ := item["vehicle_number"].(string)
				stops, _ := item["stops"].([]interface{})
				if academy != "" || vehicle != "" {
					sb.WriteString(fmt.Sprintf("🚌 %s %s\n", academy, vehicle))
				}
				for _, st := range stops {
					if sm, ok := st.(map[string]interface{}); ok {
						loc, _ := sm["location"].(string)
						schedules, _ := sm["display_schedules"].(map[string]interface{})
						if loc != "" {
							sb.WriteString(fmt.Sprintf("📍 %s\n", loc))
							for k, v := range schedules {
								if v != nil {
									sb.WriteString(fmt.Sprintf("  • %s: %v\n", k, v))
								}
							}
						}
					}
				}
				sb.WriteString("\n")
			}
			if sb.Len() > 0 {
				return strings.TrimSpace(sb.String())
			}
		}
	}

	// 2. JSON Object formatting
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &obj); err == nil {
			if msg, ok := obj["message"].(string); ok && msg != "" {
				return msg
			}
			if text, ok := obj["text"].(string); ok && text != "" {
				return text
			}
		}
	}

	return trimmed
}

