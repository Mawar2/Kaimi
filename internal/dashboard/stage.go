package dashboard

import (
	"github.com/Mawar2/Kaimi/internal/opportunity"
)

// Stage represents a derived pipeline stage for an opportunity.
// Stages are derived deterministically from Opportunity field values;
// the value is never stored directly in the database.
//
// Only Phase 0 stages are implemented here. Later stages are deferred to
// the phases that introduce the corresponding agents.
type Stage int

const (
	// StageHunted is the default stage: opportunity discovered but not yet scored.
	StageHunted Stage = iota
	// StageScored means the Scorer has run (ScoredAt is set).
	StageScored
	// TODO(phase-1): StageSelected — human selected the opportunity for proposal work
	//                (Selected=true, ProposalStatus="").
	// TODO(phase-2): StageInProposal — Zone 2 agents actively working (Selected=true,
	//                ProposalStatus != "" and not a terminal value).
	// TODO(phase-2): StageAwaitingHumanReview — Zone 2 agent flagged for human
	//                intervention (ProposalStatus ends with ":needs_human").
	// TODO(phase-3): StageFinalized — proposal ready for human submission
	//                (ProposalStatus == "final-review:ready_to_submit").
)

// String returns a human-readable label for the stage, suitable for display.
func (s Stage) String() string {
	switch s {
	case StageHunted:
		return "Hunted"
	case StageScored:
		return "Scored"
	default:
		return "Unknown"
	}
}

// DeriveStage returns the pipeline stage for an opportunity by inspecting its
// field values. The derivation is deterministic and requires no I/O.
//
// Phase 0 rules (applied in priority order; first match wins):
//  1. Scored: ScoredAt != nil
//  2. Hunted: default
func DeriveStage(opp *opportunity.Opportunity) Stage {
	if opp.ScoredAt != nil {
		return StageScored
	}
	return StageHunted
}
