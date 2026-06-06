package agent

import "time"

// Status is a typed string enum representing the outcome of an agent's execution.
type Status string

const (
	// StatusSuccess means the agent completed its work without errors.
	StatusSuccess Status = "success"

	// StatusFailed means the agent encountered an unrecoverable error.
	StatusFailed Status = "failed"

	// StatusNeedsHuman means the agent requires human intervention before
	// the pipeline can continue. The coordinator must re-invoke the agent
	// after the human resolves the ambiguity.
	StatusNeedsHuman Status = "needs_human"

	// StatusReadyToSubmit means the agent completed its work and the output
	// is ready for the human review gate before submission.
	StatusReadyToSubmit Status = "ready_to_submit"
)

// AgentResult is the single stable return type for every Kaimi agent.
//
// Locking this contract early lets all downstream agents (Scorer, Manager,
// Outline, Writer, Final Review) be built and tested independently.
//
//nolint:revive // AgentResult is intentionally explicit as a cross-package public type
type AgentResult struct {
	// AgentName identifies which agent produced this result (e.g., "hunter", "scorer").
	AgentName string `json:"agent_name"`

	// Status describes the outcome of the agent's execution.
	Status Status `json:"status"`

	// NoticeID is the SAM.gov notice ID this result is associated with.
	NoticeID string `json:"notice_id"`

	// Summary is a human-readable description of what the agent did or found.
	Summary string `json:"summary"`

	// OutputRef is an optional reference to the agent's primary output artifact,
	// such as a store key, file path, or URL.
	OutputRef string `json:"output_ref,omitempty"`

	// Flags is an extensible key-value map for agent-specific metadata.
	// Downstream agents can pass structured data here without changing the contract.
	Flags map[string]string `json:"flags,omitempty"`

	// Error contains a human-readable message when Status is StatusFailed.
	Error string `json:"error,omitempty"`

	// CompletedAt records when the agent finished execution (UTC).
	CompletedAt time.Time `json:"completed_at"`
}

// IsSuccess returns true when the agent completed its work without errors.
func (r *AgentResult) IsSuccess() bool {
	return r.Status == StatusSuccess
}

// IsFailed returns true when the agent encountered an unrecoverable error.
func (r *AgentResult) IsFailed() bool {
	return r.Status == StatusFailed
}

// NeedsHuman returns true when the agent requires human intervention to continue.
func (r *AgentResult) NeedsHuman() bool {
	return r.Status == StatusNeedsHuman
}

// IsTerminal returns true when the pipeline can advance without re-invoking this
// agent. StatusNeedsHuman is NOT terminal: the coordinator must re-invoke the
// agent after the human resolves the ambiguity.
func (r *AgentResult) IsTerminal() bool {
	return r.Status != StatusNeedsHuman
}
