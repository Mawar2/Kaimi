// Package agent defines shared types used by all Zone 2 agents (Outline, Writer,
// Final Review). Every Zone 2 agent takes an Opportunity in and returns a Result.
package agent

// Status represents the outcome of an agent's run.
type Status string

const (
	// StatusSuccess means the agent completed its work without errors.
	StatusSuccess Status = "success"
	// StatusFailed means the agent encountered an unrecoverable error.
	StatusFailed Status = "failed"
	// StatusNeedsHuman means the agent completed but flagged something for human review.
	StatusNeedsHuman Status = "needs_human"
)

// Result is the standard return type for every Zone 2 agent.
//
// The Manager reads this to decide what to do next: pass the result downstream,
// pause for human review, or handle a failure.
type Result struct {
	AgentName string   // which agent produced this result
	Status    Status   // outcome: success, failed, or needs_human
	Summary   string   // human-readable description of what happened
	OutputRef string   // pointer to the output artifact (e.g. file path, Doc URL)
	Flags     []string // optional signals to the Manager (e.g. "low_confidence")
}
