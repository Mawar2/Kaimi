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

// wordsPerPage is the conversion factor used when estimating page count from word count.
const wordsPerPage = 250

// Input holds everything the Final Review agent needs to do its job.
//
// Draft is the proposal text approved by the human reviewer. Opportunity is
// the source federal opportunity, used to verify deadline and context.
// Outline is optional; when nil, section and form checks are skipped.
type Input struct {
	// Draft is the human-approved proposal text. Must not be empty.
	Draft string

	// Opportunity is the federal opportunity this proposal responds to.
	// Must not be nil.
	Opportunity *opportunity.Opportunity

	// Outline is the structured outline produced by the Outline agent.
	// When nil, only deadline and must-have checks run.
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
// It validates that the draft is non-empty and the opportunity exists, then
// runs five checks: deadline, must_have, required_section, required_form,
// and page_limit. An expired deadline returns StatusFailed immediately; all
// other issues accumulate and return StatusNeedsHuman with details in Flags.
//
// Review returns an error only for invalid input (nil opportunity, empty draft).
// Soft failures are expressed through the AgentResult so the Manager can route
// them appropriately without crashing the pipeline.
func (a *Agent) Review(ctx context.Context, in Input) (*agent.Result, error) {
	if in.Opportunity == nil {
		return nil, fmt.Errorf("final-review: opportunity must not be nil")
	}
	if in.Draft == "" {
		return nil, fmt.Errorf("final-review: draft must not be empty")
	}

	// Hard failure: expired deadline means the proposal cannot be submitted.
	if err := checkDeadline(in.Opportunity); err != nil {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusFailed,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("proposal cannot be submitted: %v", err),
			CompletedAt: time.Now().UTC(),
		}, nil
	}

	// Collect soft issues from all content checks.
	var issues []string

	issues = append(issues, checkMustHave(in.Draft, in.Opportunity.Requirements)...)

	if in.Outline != nil {
		issues = append(issues, checkRequiredSections(in.Draft, in.Outline.Sections)...)
		issues = append(issues, checkRequiredForms(in.Draft, in.Outline.FormattingRules)...)
		issues = append(issues, checkPageLimit(in.Draft, in.Outline.FormattingRules)...)
	}

	flags := buildFlags(issues)

	if len(issues) > 0 {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusNeedsHuman,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("%d issue(s) found; human review required before submission", len(issues)),
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

// checkDeadline returns an error if the opportunity's response deadline has passed.
func checkDeadline(opp *opportunity.Opportunity) error {
	if opp.ResponseDeadline.Before(time.Now()) {
		return fmt.Errorf("response deadline %s has passed",
			opp.ResponseDeadline.Format(time.DateOnly))
	}
	return nil
}

// checkMustHave scans the draft for each keyword in requirements.
// Returns one issue string per unaddressed requirement.
func checkMustHave(draft string, requirements []string) []string {
	lower := strings.ToLower(draft)
	var issues []string
	for _, req := range requirements {
		if !strings.Contains(lower, strings.ToLower(req)) {
			issues = append(issues, fmt.Sprintf(
				"must_have: requirement %q not addressed anywhere in draft", req))
		}
	}
	return issues
}

// checkRequiredSections verifies that every section with Required=true has its
// title mentioned (case-insensitive substring match) somewhere in the draft.
func checkRequiredSections(draft string, sections []outline.Section) []string {
	lower := strings.ToLower(draft)
	var issues []string
	for _, s := range sections {
		if !s.Required {
			continue
		}
		if !strings.Contains(lower, strings.ToLower(s.Title)) {
			issues = append(issues, fmt.Sprintf(
				"required_section: section %q (id: %s) not found in draft", s.Title, s.ID))
		}
	}
	return issues
}

// checkRequiredForms confirms that each form number listed in FormattingRules
// is acknowledged (case-insensitive) somewhere in the draft text.
func checkRequiredForms(draft string, rules *outline.FormattingRules) []string {
	if rules == nil {
		return nil
	}
	lower := strings.ToLower(draft)
	var issues []string
	for _, form := range rules.RequiredForms {
		if !strings.Contains(lower, strings.ToLower(form)) {
			issues = append(issues, fmt.Sprintf(
				"required_form: form %q not acknowledged in draft", form))
		}
	}
	return issues
}

// checkPageLimit estimates the draft's page count at wordsPerPage words per page
// and returns an issue string when it exceeds the stated limit. Returns nothing
// when the page limit is unspecified or the limit value cannot be parsed.
func checkPageLimit(draft string, rules *outline.FormattingRules) []string {
	if rules == nil || rules.PageLimit == nil || !rules.PageLimit.Specified {
		return nil
	}

	// PageLimit.Value may be "25 pages" or just "25" — parse the leading integer.
	limitStr := strings.Fields(rules.PageLimit.Value)[0]
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return nil // unparseable limit; skip check
	}

	wordCount := len(strings.Fields(draft))
	estimated := (wordCount + wordsPerPage - 1) / wordsPerPage // ceiling division

	if estimated > limit {
		return []string{fmt.Sprintf(
			"page_limit: draft is approximately %d page(s) but limit is %d page(s) (~%d words)",
			estimated, limit, wordCount)}
	}
	return nil
}

// buildFlags converts a slice of issue strings into the Flags map expected by
// AgentResult: "issues_found" holds the count; "issue_N" holds each detail.
func buildFlags(issues []string) map[string]string {
	flags := map[string]string{
		"issues_found": strconv.Itoa(len(issues)),
	}
	for i, detail := range issues {
		flags[fmt.Sprintf("issue_%d", i+1)] = detail
	}
	return flags
}
