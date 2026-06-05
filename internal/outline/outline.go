// Package outline implements the Outline agent for Zone 2 of the Kaimi pipeline.
//
// The Outline agent is the first specialist agent triggered by the Manager after
// a human selects an opportunity from the scored queue. It takes the selected
// Opportunity and produces a structured proposal outline.
//
// This file is the skeleton (KAI-2). Real outline generation with Gemini is added
// in subsequent tickets (KAI-3: section structure, KAI-4: Google Docs integration).
package outline

import (
	"context"
	"fmt"
	"time"

	"github.com/Mawar2/Kaimi/internal/agent"
	"github.com/Mawar2/Kaimi/internal/opportunity"
)

const agentName = "outline"

// Agent is the Outline agent. It takes a selected Opportunity and produces a
// structured proposal outline returned as an AgentResult.
//
// Agents are stateless: construct once and call Run for each opportunity.
// TODO(phase-3): inject LLM client here when real generation is added.
type Agent struct{}

// New creates a new Outline agent.
func New() *Agent {
	return &Agent{}
}

// Run takes a selected Opportunity and produces an outline AgentResult.
//
// Returns AgentStatusFailed (with a non-nil error) when opp is nil.
// Returns AgentStatusSuccess with a stubbed summary on the happy path.
//
// TODO(phase-3): Replace stub logic with real outline generation using Gemini.
func (a *Agent) Run(ctx context.Context, opp *opportunity.Opportunity) (*agent.AgentResult, error) {
	completedAt := time.Now()

	if opp == nil {
		return &agent.AgentResult{
			AgentName:   agentName,
			Status:      agent.StatusFailed,
			Error:       "outline agent: opportunity must not be nil",
			CompletedAt: completedAt,
		}, fmt.Errorf("outline agent: opportunity must not be nil")
	}

	// Context cancellation check — don't proceed after caller gives up.
	select {
	case <-ctx.Done():
		return &agent.AgentResult{
			AgentName:   agentName,
			Status:      agent.StatusFailed,
			NoticeID:    opp.ID,
			Error:       ctx.Err().Error(),
			CompletedAt: time.Now(),
		}, ctx.Err()
	default:
	}

	// Stub: return success with a placeholder summary.
	// Real section generation is added in KAI-3.
	return &agent.AgentResult{
		AgentName:   agentName,
		Status:      agent.StatusSuccess,
		NoticeID:    opp.ID,
		Summary:     fmt.Sprintf("outline stub complete for opportunity %s: %s", opp.ID, opp.Title),
		OutputRef:   "", // TODO(phase-3): set to Google Doc URL once KAI-5 is built
		Flags:       map[string]string{"stub": "true"},
		CompletedAt: completedAt,
	}, nil
}
