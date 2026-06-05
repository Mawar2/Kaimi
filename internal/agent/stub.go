package agent

import (
	"context"
	"fmt"
	"time"
)

// StubAgent is a no-op agent used in tests and integration scaffolding.
// It returns a successful AgentResult immediately, or an error if the context
// is already cancelled when Execute is called.
//
// StubAgent is not intended for production use — it exists so downstream
// packages can depend on the agent contract without needing a live LLM.
type StubAgent struct {
	// Name is the agent identifier returned in the AgentResult.
	Name string
}

// Execute returns a successful AgentResult, or an error if ctx is cancelled.
func (a *StubAgent) Execute(ctx context.Context) (AgentResult, error) {
	if err := ctx.Err(); err != nil {
		return AgentResult{}, fmt.Errorf("agent %q: context cancelled before execution: %w", a.Name, err)
	}

	return AgentResult{
		AgentName:   a.Name,
		Status:      StatusSuccess,
		Summary:     fmt.Sprintf("StubAgent %q executed successfully (no-op).", a.Name),
		CompletedAt: time.Now().UTC(),
	}, nil
}
