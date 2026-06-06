// Package finalreview implements the Final Review agent — the last automated
// checkpoint before a human submits a proposal.
//
// The Final Review agent sits at the end of the Zone 2 pipeline:
//
//	Manager → Outline → Technical Writer → [HUMAN GATE] → Final Review
//
// It receives the human-approved draft and its Opportunity (and optionally
// the Outline), runs a set of automated checks, and returns an AgentResult
// indicating whether the proposal is ready for submission.
//
// Checks performed:
//   - deadline: expired deadline → StatusFailed (fatal, halts all other checks)
//   - must_have: each keyword in Opportunity.Requirements must appear in the draft
//   - required_section: every Required=true section title must appear in the draft
//   - required_form: every form number in FormattingRules.RequiredForms must be acknowledged
//   - page_limit: word count must not exceed the solicitation's stated page limit
//
// The Outline field on Input is optional. When nil, only deadline and must-have
// checks run — existing callers are unaffected.
//
// Issues are reported in AgentResult.Flags under "issues_found" (count) and
// "issue_N" (one detail string per issue, containing category, what, and where).
//
// IMPORTANT: This agent NEVER submits anything. StatusReadyToSubmit in the
// returned AgentResult is a signal for a human to act on. No submission API
// is called by this agent.
package finalreview
