package agent

import (
	"context"
	"fmt"
	"time"
)

// StubAgent is a minimal agent implementation proving the AgentResult contract works.
// Use it as the starting template for every real agent:
//  1. Take input (Opportunity, config, etc.)
//  2. Do work (call APIs, run LLM, etc.)
//  3. Return AgentResult with status and output location
type StubAgent struct {
	name string
}

// NewStubAgent creates a stub agent with the given name.
func NewStubAgent(name string) *StubAgent {
	return &StubAgent{name: name}
}

// Execute runs the stub agent and returns a successful AgentResult.
// It respects context cancellation, which all real agents must also do.
func (a *StubAgent) Execute(ctx context.Context, noticeID string) (*AgentResult, error) {
	select {
	case <-ctx.Done():
		return &AgentResult{
			AgentName:   a.name,
			Status:      StatusFailed,
			NoticeID:    noticeID,
			Error:       "context cancelled",
			CompletedAt: time.Now(),
		}, ctx.Err()
	case <-time.After(10 * time.Millisecond):
		// Simulate minimal work before returning.
	}

	return &AgentResult{
		AgentName:   a.name,
		Status:      StatusSuccess,
		NoticeID:    noticeID,
		Summary:     fmt.Sprintf("Stub agent '%s' completed successfully for notice %s", a.name, noticeID),
		OutputRef:   fmt.Sprintf("output/%s/%s.json", a.name, noticeID),
		Flags:       map[string]string{"stub": "true", "version": "1.0"},
		CompletedAt: time.Now(),
	}, nil
}

// ExecuteWithError simulates an agent failure. Useful for testing the Manager's
// error-handling paths without wiring a real failing external dependency.
func (a *StubAgent) ExecuteWithError(_ context.Context, noticeID, errMsg string) (*AgentResult, error) {
	return &AgentResult{
		AgentName:   a.name,
		Status:      StatusFailed,
		NoticeID:    noticeID,
		Error:       errMsg,
		CompletedAt: time.Now(),
	}, fmt.Errorf("agent failed: %s", errMsg)
}

// ExecuteNeedsHuman simulates an agent that requires human intervention.
// Useful for testing the Manager's human review gate.
func (a *StubAgent) ExecuteNeedsHuman(_ context.Context, noticeID, reason string) (*AgentResult, error) {
	return &AgentResult{
		AgentName:   a.name,
		Status:      StatusNeedsHuman,
		NoticeID:    noticeID,
		Summary:     reason,
		Flags:       map[string]string{"intervention_needed": "true"},
		CompletedAt: time.Now(),
	}, nil
}
