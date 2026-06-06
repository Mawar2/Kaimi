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
// Outline is the structured proposal outline from the Outline agent; when nil,
// section and form checks are skipped — no breaking change to existing callers.
type Input struct {
	// Draft is the human-approved proposal text. Must not be empty.
	Draft string

	// Opportunity is the federal opportunity this proposal responds to.
	// Must not be nil.
	Opportunity *opportunity.Opportunity

	// Outline is the structured outline from the Outline agent.
	// When nil, required-section and required-form checks are skipped.
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
// five automated checks: deadline, must-have requirements, required sections,
// required forms, and page limit. Issues are collected in AgentResult.Flags
// under "issues_found" (count) and "issue_N" (detail strings).
//
// Review returns an error only for invalid input (nil opportunity, empty
// draft). Soft failures — like an expired deadline — are expressed through
// the AgentResult status so the Manager can route them appropriately without
// crashing the pipeline.
func (a *Agent) Review(ctx context.Context, in Input) (*agent.Result, error) {
	// Validate required inputs at the system boundary.
	if in.Opportunity == nil {
		return nil, fmt.Errorf("final-review: opportunity must not be nil")
	}
	if in.Draft == "" {
		return nil, fmt.Errorf("final-review: draft must not be empty")
	}

	// Deadline check: hard failure — a missed deadline cannot be recovered.
	if err := checkDeadline(in.Opportunity); err != nil {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusFailed,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("proposal cannot be submitted: %v", err),
			CompletedAt: time.Now().UTC(),
		}, nil
	}

	// Collect issues from all content checks.
	var issues []string
	issues = append(issues, checkMustHave(in.Draft, in.Opportunity.Requirements)...)
	if in.Outline != nil {
		issues = append(issues, checkRequiredSections(in.Draft, in.Outline)...)
		issues = append(issues, checkRequiredForms(in.Draft, in.Outline)...)
		issues = append(issues, checkPageLimit(in.Draft, in.Outline)...)
	}

	flags := buildFlags(issues)

	if len(issues) > 0 {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusNeedsHuman,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("%d issue(s) found during automated checks", len(issues)),
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

// checkMustHave scans the draft for each keyword in requirements.
// Returns an issue string for every keyword not found (case-insensitive).
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

// checkRequiredSections verifies every Required=true section from the Outline
// has a matching title (case-insensitive substring) in the draft.
func checkRequiredSections(draft string, ol *outline.Outline) []string {
	lower := strings.ToLower(draft)
	var issues []string
	for _, sec := range ol.Sections {
		if !sec.Required {
			continue
		}
		if !strings.Contains(lower, strings.ToLower(sec.Title)) {
			issues = append(issues, fmt.Sprintf("required_section: %q not found in draft", sec.Title))
		}
	}
	return issues
}

// checkRequiredForms confirms each form number in FormattingRules.RequiredForms
// is acknowledged somewhere in the draft (case-insensitive).
func checkRequiredForms(draft string, ol *outline.Outline) []string {
	if ol.FormattingRules == nil {
		return nil
	}
	lower := strings.ToLower(draft)
	var issues []string
	for _, form := range ol.FormattingRules.RequiredForms {
		if !strings.Contains(lower, strings.ToLower(form)) {
			issues = append(issues, fmt.Sprintf("required_form: %q not acknowledged in draft", form))
		}
	}
	return issues
}

// checkPageLimit estimates page count at 250 words/page and flags when the
// draft exceeds the solicitation's stated limit. Skipped if Specified=false.
func checkPageLimit(draft string, ol *outline.Outline) []string {
	if ol.FormattingRules == nil || ol.FormattingRules.PageLimit == nil {
		return nil
	}
	rule := ol.FormattingRules.PageLimit
	if !rule.Specified {
		return nil
	}
	limit := parseFirstInt(rule.Value)
	if limit <= 0 {
		return nil
	}
	words := len(strings.Fields(draft))
	// Ceiling division: how many full pages does the draft fill?
	pages := (words + 249) / 250
	if pages > limit {
		return []string{fmt.Sprintf("page_limit: draft is ~%d pages, exceeds limit of %d", pages, limit)}
	}
	return nil
}

// parseFirstInt extracts the first positive integer from a string like "25 pages".
func parseFirstInt(s string) int {
	for _, f := range strings.Fields(s) {
		if n, err := strconv.Atoi(f); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

// buildFlags constructs the Flags map from the collected issue list.
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
