// Package finalreview implements the Final Review agent — the last automated
// checkpoint before a human submits a proposal.
//
// The Final Review agent sits at the end of the Zone 2 pipeline:
//
//	Manager → Outline → Technical Writer → [HUMAN GATE] → Final Review
//
// It receives the human-approved draft and its Opportunity, runs a set of
// automated checks (deadline still open, must-have requirements addressed,
// required sections present, required forms acknowledged, page limit respected),
// and returns an AgentResult indicating whether the proposal is ready for
// submission.
//
// IMPORTANT: This agent NEVER submits anything. ReadyToSubmit=true in the
// returned AgentResult is a signal for a human to act on. No submission API
// is called by this agent.
package finalreview
