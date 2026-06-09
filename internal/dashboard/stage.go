package dashboard

import (
	"strings"

	"github.com/Mawar2/Kaimi/internal/opportunity"
)

// Stage represents a derived pipeline stage for an opportunity.
// Stages are derived deterministically from Opportunity field values;
// the value is never stored directly in the database.
//
// See docs/dashboard/architecture.md §Stage definitions and derivation rules.
type Stage int

const (
	// StageHunted is the default stage: opportunity discovered but not yet scored.
	StageHunted Stage = iota
	// StageScored means the Scorer has run but the opportunity is not yet selected.
	StageScored
	// StageSelected means a human has selected the opportunity for proposal work,
	// but Zone 2 agents have not started yet.
	StageSelected
	// StageInProposal means Zone 2 agents are actively working on the proposal.
	StageInProposal
	// StageAwaitingHumanReview means a Zone 2 agent flagged the opportunity for
	// human intervention (ProposalStatus ends with ":needs_human").
	StageAwaitingHumanReview
	// StageFinalized means the proposal is ready for human submission
	// (ProposalStatus == "final-review:ready_to_submit").
	StageFinalized
)

// String returns a human-readable label for the stage, suitable for display.
func (s Stage) String() string {
	switch s {
	case StageHunted:
		return "Hunted"
	case StageScored:
		return "Scored"
	case StageSelected:
		return "Selected"
	case StageInProposal:
		return "In Proposal"
	case StageAwaitingHumanReview:
		return "Awaiting Human Review"
	case StageFinalized:
		return "Finalized"
	default:
		return "Unknown"
	}
}

// DeriveStage returns the pipeline stage for an opportunity by inspecting its
// field values. The derivation is deterministic and requires no I/O.
//
// Rules are applied in priority order; the first match wins:
//  1. Finalized: Selected && ProposalStatus == "final-review:ready_to_submit"
//  2. AwaitingHumanReview: Selected && ProposalStatus ends with ":needs_human"
//  3. InProposal: Selected && ProposalStatus != ""
//  4. Selected: Selected && ProposalStatus == ""
//  5. Scored: !Selected && ScoredAt != nil
//  6. Hunted: default (ScoredAt == nil)
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
