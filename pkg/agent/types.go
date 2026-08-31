package agent

// Message represents a conversation turn.
type Message struct {
	Role       string     `json:"role"` // "system", "user", "assistant", "tool"
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall represents a model-requested tool invocation.
type ToolCall struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Arguments        string `json:"arguments"`
	ThoughtSignature string `json:"thought_signature,omitempty"`
}

// ToolDefinition describes a tool schema to the LLM.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// CompletionRequest holds basic prompt parameters.
type CompletionRequest struct {
	SystemPrompt string    `json:"system_prompt,omitempty"`
	Messages     []Message `json:"messages"`
	MaxTokens    int       `json:"max_tokens,omitempty"`
	Temperature  float64   `json:"temperature,omitempty"`
}

// ToolCompletionRequest holds prompt parameters along with available tool schemas.
type ToolCompletionRequest struct {
	CompletionRequest
	Tools []ToolDefinition `json:"tools,omitempty"`
}

// CompletionResponse holds text generation output.
type CompletionResponse struct {
	Text  string     `json:"text"`
	Model string     `json:"model,omitempty"`
	Usage TokenUsage `json:"usage"`
}

// ToolCompletionResponse holds generation output with optional tool calls.
type ToolCompletionResponse struct {
	Text      string     `json:"text"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Model     string     `json:"model,omitempty"`
	Usage     TokenUsage `json:"usage"`
}

// TokenUsage holds token consumption metadata.
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ToolResult represents the execution outcome of a tool call.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"`
	Output     string `json:"output"`
	Error      error  `json:"error,omitempty"`
}

// AgentStep represents a single reasoning step in the Agent execution loop.
type AgentStep struct {
	Iteration   int          `json:"iteration"`
	Thought     string       `json:"thought,omitempty"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
}

// AgentResult represents the final execution outcome of an Agent run.
type AgentResult struct {
	Output string      `json:"output"`
	Steps  []AgentStep `json:"steps"`
	Usage  TokenUsage  `json:"usage"`
}
