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
// the source federal opportunity, used to verify deadline and must-have
// requirements. Outline is optional — when nil, only deadline and must-have
// checks run; when provided, required sections, required government forms, and
// the page-limit check also run.
type Input struct {
	// Draft is the human-approved proposal text. Must not be empty.
	Draft string

	// Opportunity is the federal opportunity this proposal responds to.
	// Must not be nil.
	Opportunity *opportunity.Opportunity

	// Outline is the structured outline from the Outline agent (KAI-4).
	// Optional. When nil, section, form, and page-limit checks are skipped.
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
// It validates that the draft is non-empty, the opportunity exists, and that
// the response deadline has not passed. When an Outline is provided it also
// checks that every Required section title appears in the draft, every
// required government form number is acknowledged, and the estimated page
// count does not exceed the stated page limit.
//
// Review returns an error only for invalid input (nil opportunity, empty
// draft). Soft failures — like an expired deadline or missing sections — are
// expressed through the AgentResult status so the Manager can route them
// appropriately without crashing the pipeline.
//
// Issues are reported in AgentResult.Flags under "issues_found" (count) and
// "issue_N" (detail string). StatusReadyToSubmit is returned only when all
// checks pass; StatusNeedsHuman when issues are found; StatusFailed only for
// an expired deadline. The agent never calls any submission API.
func (a *Agent) Review(ctx context.Context, in Input) (*agent.Result, error) {
	// Validate required inputs at the system boundary.
	if in.Opportunity == nil {
		return nil, fmt.Errorf("final-review: opportunity must not be nil")
	}
	if in.Draft == "" {
		return nil, fmt.Errorf("final-review: draft must not be empty")
	}

	// Deadline is a hard failure — expired proposals cannot be submitted.
	if err := checkDeadline(in.Opportunity); err != nil {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusFailed,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("proposal cannot be submitted: %v", err),
			CompletedAt: time.Now().UTC(),
		}, nil
	}

	var issues []string
	issues = append(issues, checkMustHaves(in.Draft, in.Opportunity)...)

	if in.Outline != nil {
		issues = append(issues, checkRequiredSections(in.Draft, in.Outline)...)
		issues = append(issues, checkRequiredForms(in.Draft, in.Outline)...)
		issues = append(issues, checkPageLimit(in.Draft, in.Outline)...)
	}

	flags := buildFlags(issues)

	if len(issues) == 0 {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusReadyToSubmit,
			NoticeID:    in.Opportunity.ID,
			Summary:     "all automated checks passed; proposal is ready for human submission",
			Flags:       flags,
			CompletedAt: time.Now().UTC(),
		}, nil
	}

	return &agent.Result{
		AgentName:   agentName,
		Status:      agent.StatusNeedsHuman,
		NoticeID:    in.Opportunity.ID,
		Summary:     fmt.Sprintf("%d issue(s) require human attention before submission", len(issues)),
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

// checkMustHaves scans the draft for each keyword in Opportunity.Requirements.
// Each requirement not found in the draft (case-insensitive) is returned as a
// must_have issue.
func checkMustHaves(draft string, opp *opportunity.Opportunity) []string {
	draftLower := strings.ToLower(draft)
	var issues []string
	for _, req := range opp.Requirements {
		if !strings.Contains(draftLower, strings.ToLower(req)) {
			issues = append(issues, fmt.Sprintf("must_have: %q not addressed in draft", req))
		}
	}
	return issues
}

// checkRequiredSections verifies that every Required section's Title appears
// in the draft (case-insensitive substring match). Optional sections are
// ignored.
func checkRequiredSections(draft string, ol *outline.Outline) []string {
	draftLower := strings.ToLower(draft)
	var issues []string
	for _, section := range ol.Sections {
		if !section.Required {
			continue
		}
		if !strings.Contains(draftLower, strings.ToLower(section.Title)) {
			issues = append(issues, fmt.Sprintf("required_section: %q missing from draft", section.Title))
		}
	}
	return issues
}

// checkRequiredForms verifies that each form number in
// FormattingRules.RequiredForms is mentioned in the draft (case-insensitive).
func checkRequiredForms(draft string, ol *outline.Outline) []string {
	if ol.FormattingRules == nil {
		return nil
	}
	draftLower := strings.ToLower(draft)
	var issues []string
	for _, form := range ol.FormattingRules.RequiredForms {
		if !strings.Contains(draftLower, strings.ToLower(form)) {
			issues = append(issues, fmt.Sprintf("required_form: %q not acknowledged in draft", form))
		}
	}
	return issues
}

// checkPageLimit estimates the draft's page count at 250 words per page and
// flags it when the count exceeds the stated page limit. Skips the check
// when PageLimit is unspecified (Specified=false).
func checkPageLimit(draft string, ol *outline.Outline) []string {
	if ol.FormattingRules == nil || ol.FormattingRules.PageLimit == nil || !ol.FormattingRules.PageLimit.Specified {
		return nil
	}
	// Value is e.g. "25 pages" — parse the leading integer.
	fields := strings.Fields(ol.FormattingRules.PageLimit.Value)
	if len(fields) == 0 {
		return nil
	}
	limit, err := strconv.Atoi(fields[0])
	if err != nil {
		return nil
	}
	words := len(strings.Fields(draft))
	// Ceiling division: every started page counts.
	estimatedPages := (words + 249) / 250
	if estimatedPages > limit {
		return []string{fmt.Sprintf(
			"page_limit: draft is ~%d pages, exceeds %d-page limit", estimatedPages, limit,
		)}
	}
	return nil
}

// buildFlags converts the issues slice into the Flags map format expected by
// AgentResult. Always sets "issues_found"; sets "issue_N" for each issue.
func buildFlags(issues []string) map[string]string {
	flags := map[string]string{
		"issues_found": strconv.Itoa(len(issues)),
	}
	for i, issue := range issues {
		flags[fmt.Sprintf("issue_%d", i+1)] = issue
	}
	return flags
}
