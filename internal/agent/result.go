// Package agent defines the shared contract that every agent in the system must conform to.
// All agents — stub or real — return an AgentResult so callers handle one type.
package agent

// AgentStatus is the outcome of an agent's execution.
// Using a typed string keeps JSON serialisation self-documenting.
type AgentStatus string

const (
	// AgentStatusSuccess means the agent completed its work without errors.
	AgentStatusSuccess AgentStatus = "success"

	// AgentStatusFailed means the agent encountered an unrecoverable error.
	AgentStatusFailed AgentStatus = "failed"

	// AgentStatusNeedsHuman means the agent requires human intervention to proceed.
	AgentStatusNeedsHuman AgentStatus = "needs_human"

	// AgentStatusReadyToSubmit means work is complete and a PR is ready for review.
	AgentStatusReadyToSubmit AgentStatus = "ready_to_submit"
)

// IsTerminal returns true when no further processing is needed after this status.
func (s AgentStatus) IsTerminal() bool {
	return s == AgentStatusSuccess || s == AgentStatusFailed || s == AgentStatusReadyToSubmit
}

// AgentResult is the return type every agent in the system must produce.
// Locking this contract here lets callers, the Manager, and the task queue all
// depend on a single stable shape regardless of which agent produced the result.
type AgentResult struct {
	// AgentName identifies which agent produced this result (e.g. "stub", "code-writer").
	AgentName string `json:"agent_name"`

	// Status is the terminal or intermediate state of the agent's execution.
	Status AgentStatus `json:"status"`

	// NoticeID is an optional correlation handle — a GitHub issue number, ticket ID,
	// or any reference that ties this result back to the triggering event.
	NoticeID string `json:"notice_id"`

	// Summary is a human-readable one-line description of what the agent did.
	Summary string `json:"summary"`

	// OutputRef is a pointer to the primary output artifact: a PR URL, file path,
	// branch name, or any location where the work product can be inspected.
	OutputRef string `json:"output_ref"`

	// Flags are zero or more labels that qualify the result
	// (e.g. "tdd_complete", "conventions_checked", "needs_escalation").
	Flags []string `json:"flags"`

	// Error holds a description of what went wrong when Status is AgentStatusFailed.
	// Empty string when the agent succeeded.
	Error string `json:"error,omitempty"`
}
