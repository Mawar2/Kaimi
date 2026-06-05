package agent

import "time"

// Status is a typed string representing the terminal or blocking state of an
// agent execution. Using a named type prevents accidentally passing an arbitrary
// string where a status is expected.
type Status string

const (
	// StatusSuccess means the agent completed its task without errors.
	StatusSuccess Status = "success"

	// StatusFailed means the agent encountered an unrecoverable error.
	StatusFailed Status = "failed"

	// StatusNeedsHuman means the agent requires a human decision before it or a
	// downstream agent can continue (e.g. reviewing a generated outline).
	StatusNeedsHuman Status = "needs_human"

	// StatusReadyToSubmit means the proposal is complete and awaiting human
	// sign-off before submission — a specialized terminal state used by Final Review.
	StatusReadyToSubmit Status = "ready_to_submit"
)

// IsSuccess reports whether the status represents a clean completion.
func (s Status) IsSuccess() bool { return s == StatusSuccess }

// IsFailed reports whether the status represents an unrecoverable failure.
func (s Status) IsFailed() bool { return s == StatusFailed }

// NeedsHuman reports whether the status requires a human decision to proceed.
func (s Status) NeedsHuman() bool { return s == StatusNeedsHuman }

// IsTerminal reports whether the pipeline can advance without human input.
// success, failed, and ready_to_submit are terminal; needs_human is not.
func (s Status) IsTerminal() bool {
	return s == StatusSuccess || s == StatusFailed || s == StatusReadyToSubmit
}

// AgentResult is the standard return type for every Zone 2 agent.
//
// Every agent produces exactly one AgentResult. The Manager reads this struct
// to decide whether to advance, pause for human review, or halt the proposal
// lifecycle. Agents never call each other directly — the Manager coordinates
// by passing results downstream.
//
//nolint:revive // agent.AgentResult stutter is intentional: callers read it as agent.AgentResult, not Result.
type AgentResult struct {
	// AgentName identifies which agent produced this result (e.g. "hunter", "scorer").
	AgentName string `json:"agent_name"`

	// Status is the outcome of the agent run.
	Status Status `json:"status"`

	// Summary is a human-readable one-paragraph description of what the agent did
	// and why it produced this status. Required for all statuses.
	Summary string `json:"summary"`

	// OutputRef is an optional pointer to the agent's primary output artifact
	// (e.g. an opportunity ID, a storage path, a document reference).
	// Nil when the agent produced no persistent output (e.g. on failure).
	OutputRef *string `json:"output_ref,omitempty"`

	// Flags carries extensible key-value metadata without requiring schema changes.
	// Examples: {"naics": "541512"}, {"error_code": "429", "retry_after": "3600"}.
	// Nil when no extra metadata is needed.
	Flags map[string]string `json:"flags,omitempty"`

	// CompletedAt is when the agent finished execution (UTC).
	CompletedAt time.Time `json:"completed_at"`
}
