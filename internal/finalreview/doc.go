// Package finalreview implements the Final Review agent — the last automated
// checkpoint before a human submits a proposal.
//
// The Final Review agent sits at the end of the Zone 2 pipeline:
//
//	Manager → Outline → Technical Writer → [HUMAN GATE] → Final Review
//
// It receives the human-approved draft and its Opportunity, runs a set of
// automated checks, and returns an AgentResult indicating whether the proposal
// is ready for submission.
//
// Checks performed (in order):
//   - Deadline: response deadline has not passed (hard failure if missed).
//   - Must-haves: every entry in Opportunity.Requirements is addressed by at
//     least one significant keyword in the draft.
//   - Required sections: every Required section from the Outline agent's output
//     appears as a title in the draft (only when an Outline is provided).
//   - Required forms: every government form listed in FormattingRules is
//     acknowledged in the draft (only when an Outline is provided).
//   - Page limit: a word-count heuristic checks the draft against the solicitation's
//     stated page limit (only when an Outline with a specified page limit is provided).
//
// IMPORTANT: This agent NEVER submits anything. StatusReadyToSubmit in the
// returned AgentResult is a signal for a human to act on. No submission API
// is ever called by this agent.
package finalreview
