package dashboard

import (
	"strings"

	"github.com/Mawar2/Kaimi/internal/opportunity"
)

// Stage represents a pipeline stage in the Kaimi dashboard.
// Stages are derived deterministically from Opportunity fields; they are never
// stored directly on the opportunity.
type Stage string

const (
	// StageHunted is the initial stage: opportunity discovered but not yet scored.
	StageHunted Stage = "Hunted"

	// StageScored is set once scoring completes (ScoredAt non-nil), regardless of
	// the Recommendation value. "REVIEW" recommendation is a sub-state badge within
	// this stage, not the same as StageAwaitingHumanReview.
	StageScored Stage = "Scored"

	// StageSelected means a human has chosen the opportunity for proposal work but
	// the Zone 2 agents have not yet started (ProposalStatus is empty).
	StageSelected Stage = "Selected"

	// StageInProposal means at least one Zone 2 agent has run (ProposalStatus is
	// non-empty) but no human intervention is currently required.
	StageInProposal Stage = "In Proposal"

	// StageAwaitingHumanReview means a Zone 2 agent has flagged the proposal for
	// human input (ProposalStatus ends with ":needs_human").
	StageAwaitingHumanReview Stage = "Awaiting Human Review"

	// StageFinalized means the proposal is ready to submit
	// (ProposalStatus == "final-review:ready_to_submit").
	StageFinalized Stage = "Finalized"
)

// DeriveStage returns the pipeline stage for opp by applying the field mapping
// defined in docs/dashboard/architecture.md. Rules are evaluated in priority
// order; the first match is authoritative.
//
// Selected==false is always authoritative: if Selected is false, ProposalStatus
// is ignored (data anomaly) and the stage is Scored or Hunted based on ScoredAt.
func DeriveStage(opp *opportunity.Opportunity) Stage {
	if opp.Selected {
		switch {
		case opp.ProposalStatus == "final-review:ready_to_submit":
			return StageFinalized
		case strings.HasSuffix(opp.ProposalStatus, ":needs_human"):
			return StageAwaitingHumanReview
		case opp.ProposalStatus != "":
			return StageInProposal
		default:
			return StageSelected
		}
	}
	if opp.ScoredAt != nil {
		return StageScored
	}
	return StageHunted
}

// CountByStage tallies how many opportunities in opps fall into each Stage.
// Stages with zero entries are omitted from the returned map.
func CountByStage(opps []*opportunity.Opportunity) map[Stage]int {
	if len(opps) == 0 {
		return map[Stage]int{}
	}
	counts := make(map[Stage]int, 6)
	for _, opp := range opps {
		counts[DeriveStage(opp)]++
	}
	return counts
}
