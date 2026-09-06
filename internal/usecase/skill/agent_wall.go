package skill

import (
	"context"

	"github.com/dh-kam/kakaotalk-dobby/pkg/agent"
)

// runAgentWithHardWall bounds an agent run even when the underlying SDK
// ignores context cancellation: the caller proceeds at the deadline while the
// abandoned call finishes in the background.
func runAgentWithHardWall(ctx context.Context, botAgent agent.Agent, utterance string, history []agent.Message) (*agent.AgentResult, error) {
	type agentOutcome struct {
		result *agent.AgentResult
		err    error
	}
	outcomeCh := make(chan agentOutcome, 1)
	go func() {
		res, err := botAgent.RunWithHistory(ctx, utterance, history)
		outcomeCh <- agentOutcome{result: res, err: err}
	}()

	select {
	case outcome := <-outcomeCh:
		return outcome.result, outcome.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
