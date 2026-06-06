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
// Outline is optional — when nil, only deadline and must-have checks run.
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
// Validates that the deadline has not passed (StatusFailed if so), that every
// must-have requirement from the Opportunity appears in the draft, and — when
// an Outline is provided — that all required sections are present, all required
// government forms are acknowledged, and the page limit is not exceeded.
//
// Issues are collected in AgentResult.Flags under "issues_found" (count) and
// "issue_N" (one-indexed detail strings). StatusNeedsHuman is returned when
// issues are found; StatusReadyToSubmit when all checks pass.
//
// Review returns an error only for invalid input (nil opportunity, empty draft).
// Soft failures are expressed through the AgentResult status so the Manager can
// route them without crashing the pipeline.
func (a *Agent) Review(ctx context.Context, in Input) (*agent.Result, error) {
	if in.Opportunity == nil {
		return nil, fmt.Errorf("final-review: opportunity must not be nil")
	}
	if in.Draft == "" {
		return nil, fmt.Errorf("final-review: draft must not be empty")
	}

	// Hard gate: expired deadline → StatusFailed.
	if err := checkDeadline(in.Opportunity); err != nil {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusFailed,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("proposal cannot be submitted: %v", err),
			CompletedAt: time.Now().UTC(),
		}, nil
	}

	// Collect content issues.
	var issues []string
	issues = append(issues, checkMustHave(in.Draft, in.Opportunity.Requirements)...)
	if in.Outline != nil {
		issues = append(issues, checkRequiredSections(in.Draft, in.Outline)...)
		issues = append(issues, checkRequiredForms(in.Draft, in.Outline)...)
		issues = append(issues, checkPageLimit(in.Draft, in.Outline)...)
	}

	flags := make(map[string]string)
	flags["issues_found"] = strconv.Itoa(len(issues))
	for i, issue := range issues {
		flags[fmt.Sprintf("issue_%d", i+1)] = issue
	}

	if len(issues) > 0 {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusNeedsHuman,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("%d check(s) require attention before submission", len(issues)),
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

// checkMustHave returns an issue string for each requirement in requirements
// that is not present (case-insensitive substring) in the draft.
func checkMustHave(draft string, requirements []string) []string {
	lower := strings.ToLower(draft)
	var issues []string
	for _, req := range requirements {
		if !strings.Contains(lower, strings.ToLower(req)) {
			issues = append(issues, fmt.Sprintf("must_have: requirement %q not addressed in draft", req))
		}
	}
	return issues
}

// checkRequiredSections returns an issue for each Required=true section in the
// outline whose Title is not found (case-insensitive substring) in the draft.
func checkRequiredSections(draft string, o *outline.Outline) []string {
	lower := strings.ToLower(draft)
	var issues []string
	for _, section := range o.Sections {
		if !section.Required {
			continue
		}
		if !strings.Contains(lower, strings.ToLower(section.Title)) {
			issues = append(issues, fmt.Sprintf("required_section: section %q not found in draft", section.Title))
		}
	}
	return issues
}

// checkRequiredForms returns an issue for each form number in
// FormattingRules.RequiredForms that is not mentioned (case-insensitive) in the draft.
func checkRequiredForms(draft string, o *outline.Outline) []string {
	if o.FormattingRules == nil {
		return nil
	}
	lower := strings.ToLower(draft)
	var issues []string
	for _, form := range o.FormattingRules.RequiredForms {
		if !strings.Contains(lower, strings.ToLower(form)) {
			issues = append(issues, fmt.Sprintf("required_form: form %q not acknowledged in draft", form))
		}
	}
	return issues
}

// checkPageLimit estimates the draft's page count at 250 words per page and
// returns an issue if it exceeds the solicitation's stated page limit.
// Returns nil when the page limit is unspecified or cannot be parsed.
func checkPageLimit(draft string, o *outline.Outline) []string {
	if o.FormattingRules == nil || o.FormattingRules.PageLimit == nil {
		return nil
	}
	if !o.FormattingRules.PageLimit.Specified {
		return nil
	}
	limit, err := parsePageCount(o.FormattingRules.PageLimit.Value)
	if err != nil || limit <= 0 {
		return nil
	}
	wordCount := len(strings.Fields(draft))
	// Ceiling division: every partial page counts.
	estimatedPages := (wordCount + 249) / 250
	if estimatedPages > limit {
		return []string{fmt.Sprintf(
			"page_limit: draft exceeds page limit of %d pages (estimated %d pages at 250 words/page)",
			limit, estimatedPages,
		)}
	}
	return nil
}

// parsePageCount extracts the page count from a value like "25 pages".
func parsePageCount(value string) (int, error) {
	var n int
	_, err := fmt.Sscanf(value, "%d", &n)
	return n, err
}
