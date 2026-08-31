package agent

import "context"

// LLMProvider abstracts language model providers (Vertex AI, Bedrock, OpenAI, etc.).
type LLMProvider interface {
	Name() string
	Generate(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	GenerateWithTools(ctx context.Context, req ToolCompletionRequest) (*ToolCompletionResponse, error)
}

// Tool defines a callable action the Agent can execute.
type Tool interface {
	Name() string
	Description() string
	ParametersSchema() map[string]interface{}
	Execute(ctx context.Context, argsJSON string) (string, error)
}

// Memory stores conversation history for context preservation.
type Memory interface {
	AddMessage(msg Message)
	GetMessages() []Message
	Clear()
}

// Agent represents an autonomous reasoning agent that can think, call tools, and respond.
type Agent interface {
	Run(ctx context.Context, input string) (*AgentResult, error)
	GetProvider() LLMProvider
	GetTools() []Tool
}
