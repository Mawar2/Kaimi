package agent

import "time"

// Status represents the completion state of an agent's run.
type Status string

const (
	// StatusSuccess means the agent completed its job without errors.
	StatusSuccess Status = "success"

	// StatusFailed means the agent encountered an unrecoverable error.
	StatusFailed Status = "failed"

	// StatusNeedsHuman means the agent paused because human judgment is required
	// before it can continue. This is NOT a terminal state — the Manager must
	// re-invoke the agent after the human resolves the ambiguity.
	StatusNeedsHuman Status = "needs_human"

	// StatusReadyToSubmit means the agent produced output that is cleared for
	// human-approved submission (Zone 2 final review gate).
	StatusReadyToSubmit Status = "ready_to_submit"
)

// AgentResult is the single stable return type for every Kaimi agent.
//
// All agents in both Zone 1 (scheduled pipeline) and Zone 2 (per-proposal
// lifecycle) return this type. Locking the contract early lets each agent be
// built and tested in isolation without cross-agent coordination.
//
//nolint:revive // AgentResult is intentionally explicit as a cross-package public type.
type AgentResult struct {
	// AgentName identifies which agent produced this result (e.g. "hunter", "scorer").
	AgentName string `json:"agent_name"`

	// Status is the terminal or paused state of this agent run.
	Status Status `json:"status"`

	// NoticeID is the SAM.gov notice ID the agent was working on.
	// Omitted when the agent is not tied to a specific notice.
	NoticeID string `json:"notice_id,omitempty"`

	// Summary is a human-readable description of what the agent did.
	// Omitted when empty.
	Summary string `json:"summary,omitempty"`

	// OutputRef is a pointer to the agent's primary output artifact (e.g., a
	// file path, queue entry key, or storage reference). Omitted when empty.
	OutputRef string `json:"output_ref,omitempty"`

	// Flags carries arbitrary key/value metadata for extensibility.
	// Downstream consumers may read these without requiring a schema change.
	// Omitted when nil or empty.
	Flags map[string]string `json:"flags,omitempty"`

	// Error holds the error message when Status is StatusFailed.
	// Omitted when empty.
	Error string `json:"error,omitempty"`

	// CompletedAt is when the agent finished (success or failure).
	CompletedAt time.Time `json:"completed_at"`
}

// IsSuccess reports whether the agent completed its job successfully.
func (r *AgentResult) IsSuccess() bool {
	return r.Status == StatusSuccess
}

// IsFailed reports whether the agent encountered an unrecoverable error.
func (r *AgentResult) IsFailed() bool {
	return r.Status == StatusFailed
}

// NeedsHuman reports whether the agent paused for human intervention.
func (r *AgentResult) NeedsHuman() bool {
	return r.Status == StatusNeedsHuman
}

// IsTerminal reports whether this result represents a final state that requires
// no further agent invocation.
//
// StatusNeedsHuman is NOT terminal — the Manager must re-invoke the agent after
// a human resolves the ambiguity. All other statuses are terminal.
func (r *AgentResult) IsTerminal() bool {
	return r.Status != StatusNeedsHuman
}
