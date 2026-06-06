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

const (
	agentName    = "final-review"
	wordsPerPage = 250
)

// Input holds everything the Final Review agent needs to do its job.
//
// Draft is the proposal text approved by the human reviewer. Opportunity is
// the source federal opportunity, used to verify deadline and requirements.
// Outline is optional; when nil, only deadline and must-have checks run —
// existing callers that omit it are unaffected.
type Input struct {
	// Draft is the human-approved proposal text. Must not be empty.
	Draft string

	// Opportunity is the federal opportunity this proposal responds to.
	// Must not be nil.
	Opportunity *opportunity.Opportunity

	// Outline is the structured outline produced by the Outline agent.
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
// It validates that the draft is non-empty, the opportunity exists, and then
// runs five checks: deadline, must-have requirements, required sections,
// required forms, and page limit. Issues are reported in AgentResult.Flags
// under "issues_found" (count) and "issue_N" (one detail string each).
//
// Review returns an error only for invalid input (nil opportunity, empty
// draft). Soft failures — like a missing section — are expressed through
// AgentResult.Status so the Manager can route them without crashing the
// pipeline.
//
// StatusFailed is returned only for an expired deadline. All other issues
// yield StatusNeedsHuman. StatusReadyToSubmit is returned only when every
// check passes.
func (a *Agent) Review(ctx context.Context, in Input) (*agent.Result, error) {
	// Validate required inputs at the system boundary.
	if in.Opportunity == nil {
		return nil, fmt.Errorf("final-review: opportunity must not be nil")
	}
	if in.Draft == "" {
		return nil, fmt.Errorf("final-review: draft must not be empty")
	}

	// Deadline failure is fatal — a late submission is invalid regardless of quality.
	if err := checkDeadline(in.Opportunity); err != nil {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusFailed,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("proposal cannot be submitted: %v", err),
			CompletedAt: time.Now().UTC(),
		}, nil
	}

	// Collect soft issues from all remaining checks.
	var issues []string
	issues = append(issues, checkMustHave(in.Opportunity, in.Draft)...)
	if in.Outline != nil {
		issues = append(issues, checkRequiredSections(in.Outline, in.Draft)...)
		issues = append(issues, checkRequiredForms(in.Outline, in.Draft)...)
		issues = append(issues, checkPageLimit(in.Outline, in.Draft)...)
	}

	// Build the flags map: issues_found count plus one issue_N key per issue.
	flags := map[string]string{
		"issues_found": strconv.Itoa(len(issues)),
	}
	for i, issue := range issues {
		flags[fmt.Sprintf("issue_%d", i+1)] = issue
	}

	if len(issues) > 0 {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusNeedsHuman,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("%d issue(s) found; proposal needs human review before submission", len(issues)),
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

// checkMustHave scans the draft for each keyword in Opportunity.Requirements.
// Returns one issue string per unaddressed requirement (case-insensitive).
func checkMustHave(opp *opportunity.Opportunity, draft string) []string {
	lowerDraft := strings.ToLower(draft)
	var issues []string
	for _, req := range opp.Requirements {
		if !strings.Contains(lowerDraft, strings.ToLower(req)) {
			issues = append(issues, fmt.Sprintf("must_have: requirement %q not found in draft", req))
		}
	}
	return issues
}

// checkRequiredSections verifies that every Required=true section from the
// Outline has a matching title in the draft (case-insensitive substring).
func checkRequiredSections(ol *outline.Outline, draft string) []string {
	lowerDraft := strings.ToLower(draft)
	var issues []string
	for _, section := range ol.Sections {
		if !section.Required {
			continue
		}
		if !strings.Contains(lowerDraft, strings.ToLower(section.Title)) {
			issues = append(issues, fmt.Sprintf("required_section: section %q not found in draft", section.Title))
		}
	}
	return issues
}

// checkRequiredForms confirms each form number in FormattingRules.RequiredForms
// is acknowledged somewhere in the draft (case-insensitive).
func checkRequiredForms(ol *outline.Outline, draft string) []string {
	if ol.FormattingRules == nil {
		return nil
	}
	lowerDraft := strings.ToLower(draft)
	var issues []string
	for _, form := range ol.FormattingRules.RequiredForms {
		if !strings.Contains(lowerDraft, strings.ToLower(form)) {
			issues = append(issues, fmt.Sprintf("required_form: form %q not acknowledged in draft", form))
		}
	}
	return issues
}

// checkPageLimit estimates page count at 250 words per page and returns an
// issue when the draft exceeds the solicitation's stated page limit.
// Returns nil when the limit is unspecified or cannot be parsed.
func checkPageLimit(ol *outline.Outline, draft string) []string {
	if ol.FormattingRules == nil || ol.FormattingRules.PageLimit == nil {
		return nil
	}
	if !ol.FormattingRules.PageLimit.Specified {
		return nil
	}
	var limit int
	if _, err := fmt.Sscanf(ol.FormattingRules.PageLimit.Value, "%d", &limit); err != nil || limit <= 0 {
		return nil
	}
	wordCount := len(strings.Fields(draft))
	estimated := (wordCount + wordsPerPage - 1) / wordsPerPage // ceiling division
	if estimated > limit {
		return []string{fmt.Sprintf(
			"page_limit: draft estimated at %d pages exceeds limit of %d pages",
			estimated, limit,
		)}
	}
	return nil
}
