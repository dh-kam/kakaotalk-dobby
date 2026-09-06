package skill

import (
	"context"
	"testing"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingAgent simulates an SDK call that ignores context cancellation.
type blockingAgent struct {
	delay time.Duration
}

func (a *blockingAgent) Run(ctx context.Context, input string) (*agent.AgentResult, error) {
	return a.RunWithHistory(ctx, input, nil)
}

func (a *blockingAgent) RunWithHistory(ctx context.Context, input string, history []agent.Message) (*agent.AgentResult, error) {
	time.Sleep(a.delay)
	return &agent.AgentResult{Output: "late answer"}, nil
}

func (a *blockingAgent) GetProvider() agent.LLMProvider { return nil }
func (a *blockingAgent) GetTools() []agent.Tool         { return nil }

func TestRunAgentWithHardWall_ReturnsAtDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	res, err := runAgentWithHardWall(ctx, &blockingAgent{delay: 300 * time.Millisecond}, "질문", nil)
	elapsed := time.Since(start)

	assert.Nil(t, res)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, 200*time.Millisecond, "must return at the deadline, not when the SDK finishes")
}

func TestRunAgentWithHardWall_PassesThroughResult(t *testing.T) {
	res, err := runAgentWithHardWall(context.Background(), &blockingAgent{delay: 10 * time.Millisecond}, "질문", nil)

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "late answer", res.Output)
}
