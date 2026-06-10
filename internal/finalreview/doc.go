// Package finalreview implements the Final Review agent — the last automated
// checkpoint before a human submits a proposal.
//
// The Final Review agent sits at the end of the Zone 2 pipeline:
//
//	Manager → Outline → Technical Writer → [HUMAN GATE] → Final Review
//
// It receives the human-approved draft and its Opportunity, runs a set of
// automated checks, and returns an AgentResult indicating whether the proposal
// is ready for submission. Five checks are performed:
//
//   - deadline: expired deadline → StatusFailed; all other issues → StatusNeedsHuman
//   - must_have: each Opportunity.Requirements keyword must appear in the draft
//   - required_section: every Required=true Outline section must appear in the draft
//   - required_form: each FormattingRules.RequiredForms entry must be acknowledged
//   - page_limit: draft word count must not exceed the stated page limit (250 words/page)
//
// When the Input.Outline field is nil, only the deadline and must_have checks
// run — existing callers without an Outline are not broken.
//
// With a ComplianceChecker configured (NewWithComplianceChecker), the agent adds
// an LLM compliance pass after the deterministic checks: it vets the draft
// against the full solicitation document set (Input.Documents — Section L/M and
// SOW deliverables) and reports any mandatory requirement the draft fails to
// address. The deterministic checks remain a cheap pre-filter that always runs
// first; the LLM pass runs only when documents are present. Both unmet
// requirements and a failure to run the check route the proposal to
// StatusNeedsHuman. The pass is grounded strictly in the provided documents — it
// never invents requirements (see compliance.go).
//
// IMPORTANT: This agent NEVER submits anything. StatusReadyToSubmit in the
// returned AgentResult is a signal for a human to act on. No submission API
// is called by this agent.
package finalreview
