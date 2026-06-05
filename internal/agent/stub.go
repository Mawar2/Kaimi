package agent

import (
	"context"
	"fmt"
	"time"
)

// StubAgent is a minimal implementation of the agent contract used to validate
// that the AgentResult shape compiles, serialises, and returns correctly.
// Use it in tests and as a template when building real agents.
//
// Real agents follow the same pattern:
//  1. Receive a noticeID (SAM.gov opportunity identifier).
//  2. Perform their specific work (call an API, run an LLM, etc.).
//  3. Return an AgentResult with the appropriate status, summary, and OutputRef.
type StubAgent struct {
	name string
}

// NewStubAgent creates a StubAgent with the given name.
func NewStubAgent(name string) *StubAgent {
	return &StubAgent{name: name}
}

// Execute runs the stub and returns a valid AgentResult to prove the contract works.
// It respects context cancellation to model real agent behaviour.
func (a *StubAgent) Execute(ctx context.Context, noticeID string) (*AgentResult, error) {
	select {
	case <-ctx.Done():
		return &AgentResult{
			AgentName:   a.name,
			Status:      StatusFailed,
			NoticeID:    noticeID,
			Error:       "context cancelled before completion",
			CompletedAt: time.Now(),
		}, ctx.Err()
	default:
	}

	return &AgentResult{
		AgentName:   a.name,
		Status:      StatusSuccess,
		NoticeID:    noticeID,
		Summary:     fmt.Sprintf("stub agent %q completed successfully for notice %s", a.name, noticeID),
		OutputRef:   fmt.Sprintf("output/%s/%s.json", a.name, noticeID),
		Flags:       map[string]string{"stub": "true"},
		CompletedAt: time.Now(),
	}, nil
}
