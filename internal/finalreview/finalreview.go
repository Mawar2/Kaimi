package finalreview

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Mawar2/Kaimi/internal/agent"
	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/outline"
)

const agentName = "final-review"

// wordsPerPage is the standard estimate for converting word count to pages.
const wordsPerPage = 250

// Input holds everything the Final Review agent needs to do its job.
//
// Draft is the proposal text approved by the human reviewer. Opportunity is
// the source federal opportunity, used to verify deadline and context.
// Outline is optional; when nil, only deadline and must-have checks run —
// no breaking change to existing callers.
type Input struct {
	// Draft is the human-approved proposal text. Must not be empty.
	Draft string

	// Opportunity is the federal opportunity this proposal responds to.
	// Must not be nil.
	Opportunity *opportunity.Opportunity

	// Outline is the structured proposal outline from the Outline agent.
	// When nil, required-section, required-form, and page-limit checks are skipped.
	Outline *outline.Outline
}

// Agent is the Final Review agent.
//
// It performs automated pre-submission checks and returns an AgentResult
// indicating whether the proposal is ready for a human to submit.
// Instantiate with New().
type Agent struct{}

// New returns a new Final Review agent.
func New() *Agent {
	return &Agent{}
}

// Review runs the final automated checks on an approved proposal draft.
//
// Checks run:
//   - deadline: expired deadline → StatusFailed; cannot be submitted.
//   - must_have: each keyword in Opportunity.Requirements must appear in the draft.
//   - required_section: every Required=true section in Outline must be present.
//   - required_form: every form number in FormattingRules.RequiredForms must be mentioned.
//   - page_limit: draft word count / 250 must not exceed the stated page limit.
//
// Soft failures (must_have, section, form, page) are expressed through
// StatusNeedsHuman so the Manager can route them without crashing the pipeline.
// Issues are reported in AgentResult.Flags under "issues_found" and "issue_N".
//
// Review returns an error only for invalid input (nil opportunity, empty draft).
//
// IMPORTANT: This agent never submits anything. StatusReadyToSubmit is a signal
// for a human to act on — no submission API is called.
func (a *Agent) Review(ctx context.Context, in Input) (*agent.Result, error) {
	// Validate required inputs at the system boundary.
	if in.Opportunity == nil {
		return nil, fmt.Errorf("final-review: opportunity must not be nil")
	}
	if in.Draft == "" {
		return nil, fmt.Errorf("final-review: draft must not be empty")
	}

	// Deadline check: hard failure — cannot submit after deadline.
	if err := checkDeadline(in.Opportunity); err != nil {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusFailed,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("proposal cannot be submitted: %v", err),
			CompletedAt: time.Now().UTC(),
		}, nil
	}

	// Collect soft issues from content checks.
	var issues []string

	issues = append(issues, checkMustHave(in.Draft, in.Opportunity.Requirements)...)

	if in.Outline != nil {
		issues = append(issues, checkRequiredSections(in.Draft, in.Outline.Sections)...)
		if in.Outline.FormattingRules != nil {
			issues = append(issues, checkRequiredForms(in.Draft, in.Outline.FormattingRules.RequiredForms)...)
			issues = append(issues, checkPageLimit(in.Draft, in.Outline.FormattingRules.PageLimit)...)
		}
	}

	flags := buildFlags(issues)

	if len(issues) > 0 {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusNeedsHuman,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("%d issue(s) found in draft; human review required", len(issues)),
			Flags:       flags,
			CompletedAt: time.Now().UTC(),
		}, nil
	}

	return &agent.Result{
		AgentName:   agentName,
		Status:      agent.StatusReadyToSubmit,
		NoticeID:    in.Opportunity.ID,
		Summary:     "all automated checks passed; proposal is ready for human submission",
		Flags:       flags,
		CompletedAt: time.Now().UTC(),
	}, nil
}

// buildFlags converts a slice of issue strings into the Flags map expected by AgentResult.
// Always sets "issues_found"; adds "issue_N" keys for each issue.
func buildFlags(issues []string) map[string]string {
	flags := map[string]string{
		"issues_found": strconv.Itoa(len(issues)),
	}
	for i, iss := range issues {
		flags[fmt.Sprintf("issue_%d", i+1)] = iss
	}
	return flags
}

// checkMustHave scans draft for each keyword in requirements. Returns an issue
// string for each requirement not found (case-insensitive).
func checkMustHave(draft string, requirements []string) []string {
	lower := strings.ToLower(draft)
	var issues []string
	for _, req := range requirements {
		if !strings.Contains(lower, strings.ToLower(req)) {
			issues = append(issues, fmt.Sprintf("must_have: %q not found in draft", req))
		}
	}
	return issues
}

// checkRequiredSections verifies every Required=true section has a matching title
// in the draft (case-insensitive substring match). Optional sections are ignored.
func checkRequiredSections(draft string, sections []outline.Section) []string {
	lower := strings.ToLower(draft)
	var issues []string
	for _, s := range sections {
		if !s.Required {
			continue
		}
		if !strings.Contains(lower, strings.ToLower(s.Title)) {
			issues = append(issues, fmt.Sprintf("required_section: %q not found in draft", s.Title))
		}
	}
	return issues
}

// checkRequiredForms confirms each form number from FormattingRules.RequiredForms
// is acknowledged somewhere in the draft (case-insensitive).
func checkRequiredForms(draft string, forms []string) []string {
	lower := strings.ToLower(draft)
	var issues []string
	for _, form := range forms {
		if !strings.Contains(lower, strings.ToLower(form)) {
			issues = append(issues, fmt.Sprintf("required_form: %q not found in draft", form))
		}
	}
	return issues
}

// checkPageLimit estimates page count at 250 words/page and returns an issue
// string when the draft exceeds the stated limit. Skipped when Specified=false.
func checkPageLimit(draft string, pageLimit *outline.FormattingRule) []string {
	if pageLimit == nil || !pageLimit.Specified {
		return nil
	}
	fields := strings.Fields(pageLimit.Value)
	if len(fields) == 0 {
		return nil
	}
	limit, err := strconv.Atoi(fields[0])
	if err != nil || limit <= 0 {
		return nil
	}
	wordCount := len(strings.Fields(draft))
	if wordCount > limit*wordsPerPage {
		estimated := (wordCount + wordsPerPage - 1) / wordsPerPage
		return []string{fmt.Sprintf("page_limit: draft is %d pages, limit is %d", estimated, limit)}
	}
	return nil
}

// checkDeadline returns an error if the opportunity's response deadline has
// already passed. A proposal submitted after the deadline is invalid.
func checkDeadline(opp *opportunity.Opportunity) error {
	if opp.ResponseDeadline.Before(time.Now()) {
		return fmt.Errorf("response deadline %s has passed",
			opp.ResponseDeadline.Format(time.DateOnly))
	}
	return nil
}
