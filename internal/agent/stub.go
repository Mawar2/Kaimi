package agent

import (
	"context"
	"time"
)

// StubAgent is a minimal agent implementation that satisfies the AgentResult
// contract. It is intended as a copy-paste template for building real agents,
// and as a compile-time proof that the contract is complete and usable.
//
// Real agents should replace the stub body of Run with their actual logic,
// keeping the same signature and return type.
type StubAgent struct {
	name string
}

// NewStubAgent creates a StubAgent with the given agent name.
func NewStubAgent(name string) *StubAgent {
	return &StubAgent{name: name}
}

// Run executes the stub agent for the given SAM.gov notice ID.
//
// It respects context cancellation: if ctx is done before Run completes, it
// returns a wrapped context error immediately.
//
// Real agents should follow this same pattern — check ctx.Err() at meaningful
// checkpoints so the Manager can time out or cancel long-running work.
func (a *StubAgent) Run(ctx context.Context, noticeID string) (AgentResult, error) {
	// Check for cancellation before doing any work.
	if err := ctx.Err(); err != nil {
		return AgentResult{}, err
	}

	// TODO: Replace stub body with real agent logic.
	return AgentResult{
		AgentName:   a.name,
		Status:      StatusSuccess,
		NoticeID:    noticeID,
		Summary:     "stub agent completed successfully",
		CompletedAt: time.Now(),
	}, nil
}
