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
// the source federal opportunity, used to verify deadline and requirements.
// Outline is optional — when nil, section and form checks are skipped.
type Input struct {
	// Draft is the human-approved proposal text. Must not be empty.
	Draft string

	// Opportunity is the federal opportunity this proposal responds to.
	// Must not be nil.
	Opportunity *opportunity.Opportunity

	// Outline is the structured outline produced by the Outline agent.
	// Optional: when nil, required-section and required-form checks are skipped
	// and only deadline and must-have checks run.
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
// runs five checks: deadline, must-have requirements, required sections,
// required forms, and page limit. An expired deadline returns StatusFailed.
// Any other check failure returns StatusNeedsHuman with details in Flags.
//
// Review returns an error only for invalid input (nil opportunity, empty draft).
// Soft failures are expressed through the AgentResult status so the Manager
// can route them without crashing the pipeline.
func (a *Agent) Review(ctx context.Context, in Input) (*agent.Result, error) {
	// Validate required inputs at the system boundary.
	if in.Opportunity == nil {
		return nil, fmt.Errorf("final-review: opportunity must not be nil")
	}
	if in.Draft == "" {
		return nil, fmt.Errorf("final-review: draft must not be empty")
	}

	// Expired deadline is a hard failure — the proposal cannot be submitted at all.
	if err := checkDeadline(in.Opportunity); err != nil {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusFailed,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("proposal cannot be submitted: %v", err),
			CompletedAt: time.Now().UTC(),
		}, nil
	}

	// Collect soft issues. Each entry is a human-readable description with
	// category, what is missing, and where to look (the draft).
	var issues []string
	issues = append(issues, checkMustHave(in.Opportunity, in.Draft)...)
	if in.Outline != nil {
		issues = append(issues, checkRequiredSections(in.Outline, in.Draft)...)
		issues = append(issues, checkRequiredForms(in.Outline, in.Draft)...)
		issues = append(issues, checkPageLimit(in.Outline, in.Draft)...)
	}

	status := agent.StatusReadyToSubmit
	summary := "all automated checks passed; proposal is ready for human submission"
	if len(issues) > 0 {
		status = agent.StatusNeedsHuman
		summary = fmt.Sprintf("%d issue(s) require human attention before submission", len(issues))
	}

	return &agent.Result{
		AgentName:   agentName,
		Status:      status,
		NoticeID:    in.Opportunity.ID,
		Summary:     summary,
		Flags:       buildFlags(issues),
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

// checkMustHave returns an issue string for each entry in opp.Requirements
// that does not appear (case-insensitive substring) in the draft.
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

// checkRequiredSections returns an issue string for each Required=true section
// in the outline whose Title is not found (case-insensitive substring) in the draft.
func checkRequiredSections(ol *outline.Outline, draft string) []string {
	lowerDraft := strings.ToLower(draft)
	var issues []string
	for _, sec := range ol.Sections {
		if !sec.Required {
			continue
		}
		if !strings.Contains(lowerDraft, strings.ToLower(sec.Title)) {
			issues = append(issues, fmt.Sprintf("required_section: section %q not found in draft", sec.Title))
		}
	}
	return issues
}

// checkRequiredForms returns an issue string for each form number in
// FormattingRules.RequiredForms that is not found (case-insensitive) in the draft.
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

// checkPageLimit returns an issue string if the draft exceeds the page limit
// stated in the outline's formatting rules. Returns nothing when the limit is
// unspecified or unparseable. Estimates page count at 250 words per page
// using ceiling division.
func checkPageLimit(ol *outline.Outline, draft string) []string {
	if ol.FormattingRules == nil || ol.FormattingRules.PageLimit == nil {
		return nil
	}
	rule := ol.FormattingRules.PageLimit
	if !rule.Specified {
		return nil
	}
	// Parse limit from Value, e.g. "25 pages" → 25.
	var limitPages int
	if _, err := fmt.Sscanf(rule.Value, "%d", &limitPages); err != nil || limitPages <= 0 {
		return nil
	}
	const wordsPerPage = 250
	wordCount := len(strings.Fields(draft))
	// Ceiling division: (n + d - 1) / d
	estimatedPages := (wordCount + wordsPerPage - 1) / wordsPerPage
	if estimatedPages > limitPages {
		return []string{fmt.Sprintf(
			"page_limit: draft is ~%d pages (estimated at 250 words/page), exceeds limit of %d pages stated in draft",
			estimatedPages, limitPages,
		)}
	}
	return nil
}

// buildFlags packages the collected issues into the Flags map for AgentResult.
// Always sets "issues_found" to the count. Sets "issue_N" for each issue.
func buildFlags(issues []string) map[string]string {
	flags := map[string]string{
		"issues_found": strconv.Itoa(len(issues)),
	}
	for i, issue := range issues {
		flags[fmt.Sprintf("issue_%d", i+1)] = issue
	}
	return flags
}
