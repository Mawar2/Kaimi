package agent

import (
	"context"
	"time"
)

// StubAgent is a minimal Agent implementation that proves the AgentResult contract
// compiles and runs end-to-end. Use it as a starting template for new agents:
// copy this file, rename the type, and replace the Run body with real logic.
type StubAgent struct {
	// Name is the agent identifier included in every AgentResult.
	Name string
}

// Run executes the stub agent for the given SAM.gov notice ID.
// It returns a successful AgentResult immediately, after checking for
// context cancellation. Real agents replace this body with actual work.
func (a *StubAgent) Run(ctx context.Context, noticeID string) (*AgentResult, error) {
	// Respect context cancellation before doing any work.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return &AgentResult{
		AgentName:   a.Name,
		Status:      StatusSuccess,
		NoticeID:    noticeID,
		Summary:     "stub agent completed successfully",
		CompletedAt: time.Now().UTC(),
	}, nil
}
