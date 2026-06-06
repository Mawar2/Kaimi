// Package outline implements the Outline agent for Zone 2 of the Kaimi pipeline.
//
// The Outline agent is responsible for generating a structured proposal outline
// from a selected Opportunity. It is the first agent triggered by the Manager
// after a human selects an opportunity from the queue.
//
// This file is the skeleton (KAI-2). Real outline generation logic is added in
// subsequent tickets (KAI-3: section structure, KAI-4: formatting rules).
package outline

import (
	"context"
	"fmt"

	"github.com/Mawar2/Kaimi/internal/agent"
	"github.com/Mawar2/Kaimi/internal/opportunity"
)

const agentName = "outline"

// Agent is the Outline agent. It takes a selected Opportunity and returns a
// structured proposal outline as an AgentResult.
type Agent struct{}

// New creates a new Outline agent.
func New() *Agent {
	return &Agent{}
}

// Run takes a selected Opportunity and produces an outline AgentResult.
//
// TODO(phase-3): Replace stub logic with real outline generation using Gemini.
func (a *Agent) Run(ctx context.Context, opp *opportunity.Opportunity) (*agent.Result, error) {
	if opp == nil {
		return &agent.Result{
			AgentName: agentName,
			Status:    agent.StatusFailed,
			Summary:   "opportunity must not be nil",
		}, fmt.Errorf("outline agent: opportunity must not be nil")
	}

	// Stub: return success with a placeholder summary.
	// Real section generation is added in KAI-3.
	return &agent.Result{
		AgentName: agentName,
		Status:    agent.StatusSuccess,
		Summary:   fmt.Sprintf("outline stub complete for opportunity %s: %s", opp.ID, opp.Title),
		OutputRef: "", // TODO(phase-3): set to Google Doc URL once KAI-5 is built
		Flags:     nil,
	}, nil
}
