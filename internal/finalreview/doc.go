// Package finalreview implements the Final Review agent — the last automated
// checkpoint before a human submits a proposal.
//
// The Final Review agent sits at the end of the Zone 2 pipeline:
//
//	Manager → Outline → Technical Writer → [HUMAN GATE] → Final Review
//
// It receives the human-approved draft and its Opportunity, runs a set of
// automated checks (deadline still open, draft non-empty, required sections
// present), and returns an AgentResult indicating whether the proposal is
// ready for submission.
//
// IMPORTANT: This agent NEVER submits anything. ReadyToSubmit=true in the
// returned AgentResult is a signal for a human to act on. No submission API
// is called by this agent.
//
// Current phase (skeleton): checks are stubbed. Real LLM-backed checks
// arrive in the next ticket (KAI-7).
package finalreview
