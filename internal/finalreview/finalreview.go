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
	// When nil, required_section and required_form checks are skipped.
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
// It validates that the draft is non-empty, the opportunity exists, and runs
// five checks: deadline, must_have, required_section, required_form, and
// page_limit. Issues are reported in Flags under "issues_found" (count) and
// "issue_N" (detail string). StatusReadyToSubmit is returned only when all
// checks pass. An expired deadline returns StatusFailed; all other issues
// return StatusNeedsHuman.
//
// Review returns an error only for invalid input (nil opportunity, empty
// draft). Soft failures are expressed through the AgentResult status.
func (a *Agent) Review(ctx context.Context, in Input) (*agent.Result, error) {
	// Validate required inputs at the system boundary.
	if in.Opportunity == nil {
		return nil, fmt.Errorf("final-review: opportunity must not be nil")
	}
	if in.Draft == "" {
		return nil, fmt.Errorf("final-review: draft must not be empty")
	}

	// Deadline check: hard failure — a past deadline makes submission impossible.
	if err := checkDeadline(in.Opportunity); err != nil {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusFailed,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("proposal cannot be submitted: %v", err),
			Flags:       map[string]string{"issues_found": "0"},
			CompletedAt: time.Now().UTC(),
		}, nil
	}

	// Collect all issues across the remaining checks.
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

// checkDeadline returns an error if the opportunity's response deadline has
// already passed. A proposal submitted after the deadline is invalid.
func checkDeadline(opp *opportunity.Opportunity) error {
	if opp.ResponseDeadline.Before(time.Now()) {
		return fmt.Errorf("response deadline %s has passed",
			opp.ResponseDeadline.Format(time.DateOnly))
	}
	return nil
}

// checkMustHave scans the draft for each keyword listed in requirements.
// Returns one issue string per requirement not found in the draft.
func checkMustHave(draft string, requirements []string) []string {
	draftLower := strings.ToLower(draft)
	var issues []string
	for _, req := range requirements {
		if !strings.Contains(draftLower, strings.ToLower(req)) {
			issues = append(issues, fmt.Sprintf(
				"must_have: requirement %q not addressed in draft", req,
			))
		}
	}
	return issues
}

// checkRequiredSections verifies every Required section from the outline has a
// matching title (case-insensitive substring) in the draft.
func checkRequiredSections(draft string, sections []outline.Section) []string {
	draftLower := strings.ToLower(draft)
	var issues []string
	for _, sec := range sections {
		if !sec.Required {
			continue
		}
		if !strings.Contains(draftLower, strings.ToLower(sec.Title)) {
			issues = append(issues, fmt.Sprintf(
				"required_section: section %q not found in draft", sec.Title,
			))
		}
	}
	return issues
}

// checkRequiredForms confirms each form number in requiredForms is mentioned
// in the draft (case-insensitive).
func checkRequiredForms(draft string, requiredForms []string) []string {
	draftLower := strings.ToLower(draft)
	var issues []string
	for _, form := range requiredForms {
		if !strings.Contains(draftLower, strings.ToLower(form)) {
			issues = append(issues, fmt.Sprintf(
				"required_form: form %q not acknowledged in draft", form,
			))
		}
	}
	return issues
}

// checkPageLimit estimates the draft's page count at 250 words per page and
// flags it when the draft exceeds the solicitation's stated limit.
// Returns nil when the page limit is unspecified (Specified=false).
func checkPageLimit(draft string, limit *outline.FormattingRule) []string {
	if limit == nil || !limit.Specified {
		return nil
	}

	// Parse the page limit integer from the Value field (e.g. "25 pages").
	maxPages := parsePageCount(limit.Value)
	if maxPages <= 0 {
		return nil
	}

	wordCount := len(strings.Fields(draft))
	estimatedPages := wordCount / 250
	if wordCount%250 != 0 {
		estimatedPages++
	}

	if estimatedPages > maxPages {
		return []string{fmt.Sprintf(
			"page_limit: draft is ~%d pages (estimated at 250 words/page) but solicitation allows %d pages",
			estimatedPages, maxPages,
		)}
	}
	return nil
}

// parsePageCount extracts the leading integer from a page-limit string such as
// "25 pages" or "1 pages". Returns 0 when no integer can be parsed.
func parsePageCount(value string) int {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0
	}
	n, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0
	}
	return n
}

// buildFlags converts a slice of issue strings into the Flags map expected by
// agent.Result. Always sets "issues_found"; adds "issue_1", "issue_2", …
// for each issue.
func buildFlags(issues []string) map[string]string {
	flags := map[string]string{
		"issues_found": strconv.Itoa(len(issues)),
	}
	for i, issue := range issues {
		flags[fmt.Sprintf("issue_%d", i+1)] = issue
	}
	return flags
}
