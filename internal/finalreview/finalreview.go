package finalreview

import (
	"context"
	"fmt"
	"time"

	"github.com/Mawar2/Kaimi/internal/agent"
	"github.com/Mawar2/Kaimi/internal/opportunity"
)

const agentName = "final-review"

// Input holds everything the Final Review agent needs to do its job.
//
// Draft is the proposal text approved by the human reviewer. Opportunity is
// the source federal opportunity, used to verify deadline and context.
type Input struct {
	// Draft is the human-approved proposal text. Must not be empty.
	Draft string

	// Opportunity is the federal opportunity this proposal responds to.
	// Must not be nil.
	Opportunity *opportunity.Opportunity
}

// Agent is the Final Review agent.
//
// It performs automated pre-submission checks and returns an AgentResult
// indicating whether the proposal is ready for a human to submit.
// Instantiate with New().
type Agent struct{}

// New returns a new Final Review agent.
func New() *Agent {
	return &Agent{}
}

// Review runs the final automated checks on an approved proposal draft.
//
// It validates that the draft is non-empty, the opportunity exists, and that
// the response deadline has not passed. All checks are currently stubbed;
// LLM-backed content checks arrive in the next ticket (KAI-7).
//
// Review returns an error only for invalid input (nil opportunity, empty
// draft). Soft failures — like an expired deadline — are expressed through
// the AgentResult status so the Manager can route them appropriately without
// crashing the pipeline.
func (a *Agent) Review(ctx context.Context, in Input) (*agent.Result, error) {
	// Validate required inputs at the system boundary.
	if in.Opportunity == nil {
		return nil, fmt.Errorf("final-review: opportunity must not be nil")
	}
	if in.Draft == "" {
		return nil, fmt.Errorf("final-review: draft must not be empty")
	}

	// Run each check. Collect the first failure, if any.
	if err := checkDeadline(in.Opportunity); err != nil {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusFailed,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("proposal cannot be submitted: %v", err),
			CompletedAt: time.Now().UTC(),
		}, nil
	}

	// TODO(KAI-7): Replace stub checks with LLM-backed content review
	// (required sections present, no placeholder text, consistent tone).

	return &agent.Result{
		AgentName:   agentName,
		Status:      agent.StatusReadyToSubmit,
		NoticeID:    in.Opportunity.ID,
		Summary:     "all automated checks passed; proposal is ready for human submission",
		CompletedAt: time.Now().UTC(),
	}, nil
}

// checkDeadline returns an error if the opportunity's response deadline has
// already passed. A proposal submitted after the deadline is invalid.
func checkDeadline(opp *opportunity.Opportunity) error {
	if opp.ResponseDeadline.Before(time.Now()) {
		return fmt.Errorf("response deadline %s has passed",
			opp.ResponseDeadline.Format(time.DateOnly))
	}
	return nil
}
