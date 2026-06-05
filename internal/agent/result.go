// Package agent defines the shared contract that every Kaimi agent must conform to.
//
// The AgentResult type is the standardised return value for every agent in both
// Zone 1 (scheduled pipeline: Hunter, Scorer) and Zone 2 (per-proposal
// orchestration: Outline, Writer, Final Review). Locking this contract here
// unblocks all downstream agent tickets — callers depend on one stable shape
// regardless of which agent produced the result.
package agent

import "time"

// Status represents the outcome of an agent's execution.
// Using a typed string keeps JSON serialisation self-documenting.
type Status string

const (
	// StatusSuccess indicates the agent completed its work without errors.
	StatusSuccess Status = "success"

	// StatusFailed indicates the agent encountered an unrecoverable error.
	// The AgentResult.Error field will contain the failure details.
	StatusFailed Status = "failed"

	// StatusNeedsHuman indicates the agent requires human intervention to proceed.
	// Used when the agent detects ambiguity, conflicting requirements, or needs
	// clarification that cannot be resolved programmatically.
	StatusNeedsHuman Status = "needs_human"

	// StatusReadyToSubmit indicates the Final Review agent has approved the proposal
	// and it is ready for human review and submission to SAM.gov.
	// Only used by the Final Review agent in Zone 2.
	StatusReadyToSubmit Status = "ready_to_submit"
)

// AgentResult is the standardised return type for all Kaimi agents.
//
// Every agent — Hunter, Scorer, Outline, Writer, Final Review — returns an
// AgentResult to communicate its outcome, output location, and any metadata.
// This uniform shape lets the Manager coordinate agents without tight coupling:
// it reads one agent's AgentResult and feeds the next agent accordingly.
//
// Example usage (Scorer agent):
//
//	result := &agent.AgentResult{
//	    AgentName:   "scorer",
//	    Status:      agent.StatusSuccess,
//	    NoticeID:    "ABC-123-2026",
//	    Summary:     "Scored 87/100 — strong NAICS match, relevant past performance",
//	    OutputRef:   "opportunities/ABC-123-2026.json",
//	    Flags:       map[string]string{"score": "87", "recommendation": "BID"},
//	    CompletedAt: time.Now(),
//	}
//
// Example usage (agent failure):
//
//	result := &agent.AgentResult{
//	    AgentName:   "hunter",
//	    Status:      agent.StatusFailed,
//	    Error:       "SAM.gov API returned 429 (rate limit exceeded)",
//	    CompletedAt: time.Now(),
//	}
//
//revive:disable:exported
type AgentResult struct {
	// AgentName identifies which agent produced this result.
	// Examples: "hunter", "scorer", "outline", "writer", "final-review".
	AgentName string `json:"agent_name"`

	// Status is the outcome of the agent's execution.
	Status Status `json:"status"`

	// NoticeID is the SAM.gov notice/opportunity ID this result relates to.
	// Empty for agents that operate across multiple opportunities.
	NoticeID string `json:"notice_id,omitempty"`

	// Summary is a human-readable one-line description of what the agent did.
	// Required for StatusSuccess and StatusNeedsHuman; empty on StatusFailed.
	Summary string `json:"summary,omitempty"`

	// OutputRef points to where the agent's primary output is stored.
	// Format depends on the agent:
	//   - Hunter/Scorer: file path to the updated Opportunity JSON
	//   - Outline: Google Docs URL or file path
	//   - Final Review: validation report file path
	// Empty when Status is StatusFailed.
	OutputRef string `json:"output_ref,omitempty"`

	// Flags are extensible key-value metadata for agent-specific information.
	// They allow agents to communicate structured data without schema changes.
	// Examples:
	//   - Scorer: {"score": "87", "recommendation": "BID"}
	//   - Outline: {"section_count": "5", "doc_id": "abc123"}
	//   - Final Review: {"issues_found": "3", "ready": "false"}
	Flags map[string]string `json:"flags,omitempty"`

	// Error describes what went wrong when Status is StatusFailed.
	// Should include enough context for debugging (what failed and why).
	// Empty for non-failed statuses.
	Error string `json:"error,omitempty"`

	// CompletedAt records when the agent finished execution.
	CompletedAt time.Time `json:"completed_at"`
}

// IsSuccess returns true when the agent completed its work successfully.
func (r *AgentResult) IsSuccess() bool {
	return r.Status == StatusSuccess || r.Status == StatusReadyToSubmit
}

// IsFailed returns true when the agent could not complete its work.
func (r *AgentResult) IsFailed() bool {
	return r.Status == StatusFailed
}

// NeedsHuman returns true when the agent requires a human to intervene.
func (r *AgentResult) NeedsHuman() bool {
	return r.Status == StatusNeedsHuman
}

// IsTerminal returns true when no further processing is needed for this result.
// Both success and failure are terminal; NeedsHuman is non-terminal.
func (r *AgentResult) IsTerminal() bool {
	return r.Status == StatusSuccess || r.Status == StatusFailed || r.Status == StatusReadyToSubmit
}
