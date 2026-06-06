package outline

import (
	"fmt"
	"time"

	"github.com/Mawar2/Kaimi/internal/agent"
	"github.com/Mawar2/Kaimi/internal/opportunity"
)

// Agent generates proposal outlines from enriched opportunities.
//
// Agent is stateless and safe for concurrent use. The Manager (Zone 2
// coordinator) owns orchestration; Agent only transforms a single
// Opportunity into an Outline.
//
// TODO(phase-3): Wire in an LLM backend (Gemini Pro) to enrich sections
// with AI-generated key points, page guidance, and evaluation criteria
// extracted from the opportunity attachments.
type Agent struct{}

// NewAgent creates a new outline Agent.
func NewAgent() *Agent {
	return &Agent{}
}

// Run generates a proposal outline from the given opportunity.
//
// Returns a non-nil Outline and a success Result when opp is valid.
// Returns a nil Outline and a failed Result when opp is nil; this is not
// treated as an error because a nil opportunity is an expected input guard,
// not an unexpected system failure. Errors are reserved for I/O failures.
func (a *Agent) Run(opp *opportunity.Opportunity) (*Outline, *agent.Result, error) {
	start := time.Now()

	if opp == nil {
		return nil, &agent.Result{
			AgentName: "outline",
			Status:    "failed",
			Summary:   "opportunity is nil",
			Duration:  time.Since(start),
		}, nil
	}

	sections := buildSections(opp)
	out := &Outline{
		OpportunityID: opp.ID,
		Sections:      sections,
		GeneratedAt:   time.Now(),
	}

	return out, &agent.Result{
		AgentName: "outline",
		Status:    "success",
		Summary:   fmt.Sprintf("generated %d sections for opportunity %s", len(sections), opp.ID),
		Duration:  time.Since(start),
	}, nil
}
