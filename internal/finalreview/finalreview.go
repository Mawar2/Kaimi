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
// Outline is optional; when nil, only the deadline and must-have checks run.
type Input struct {
	// Draft is the human-approved proposal text. Must not be empty.
	Draft string

	// Opportunity is the federal opportunity this proposal responds to.
	// Must not be nil.
	Opportunity *opportunity.Opportunity

	// Outline is the structured outline produced by the Outline agent.
	// When nil, section and form checks are skipped — no breaking change
	// for callers that do not yet pass an outline.
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
// Expired deadline → StatusFailed (hard stop; proposal cannot be submitted).
// Any other failing check → StatusNeedsHuman with issues reported in Flags.
// All checks pass → StatusReadyToSubmit.
//
// Review returns an error only for invalid input (nil opportunity, empty draft).
// Soft failures are expressed through the AgentResult status so the Manager
// can route them without crashing the pipeline.
func (a *Agent) Review(ctx context.Context, in Input) (*agent.Result, error) {
	if in.Opportunity == nil {
		return nil, fmt.Errorf("final-review: opportunity must not be nil")
	}
	if in.Draft == "" {
		return nil, fmt.Errorf("final-review: draft must not be empty")
	}

	// Hard failure: expired deadline.
	if err := checkDeadline(in.Opportunity); err != nil {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusFailed,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("proposal cannot be submitted: %v", err),
			CompletedAt: time.Now().UTC(),
		}, nil
	}

	// Soft checks: collect issues; each unaddressed item is reported in Flags.
	var issues []checkIssue
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
			Summary:     fmt.Sprintf("%d check(s) require human review before submission", len(issues)),
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

// checkIssue captures a single failing check for reporting in Flags.
type checkIssue struct {
	category string // e.g. "must_have", "required_section"
	what     string // the thing that failed (e.g. the requirement keyword)
	where    string // location context (e.g. "draft")
}

func (i checkIssue) String() string {
	return fmt.Sprintf("[%s] %q not addressed in %s", i.category, i.what, i.where)
}

// buildFlags converts a slice of issues into the Flags map.
// issues_found is always set (even "0" for a clean pass) so callers can
// rely on its presence without a nil guard.
func buildFlags(issues []checkIssue) map[string]string {
	flags := map[string]string{
		"issues_found": strconv.Itoa(len(issues)),
	}
	for i, iss := range issues {
		flags[fmt.Sprintf("issue_%d", i+1)] = iss.String()
	}
	return flags
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

// checkMustHave scans the draft for each keyword in Requirements.
// Any keyword absent from the draft (case-insensitive) is flagged as an issue.
func checkMustHave(draft string, requirements []string) []checkIssue {
	lower := strings.ToLower(draft)
	var issues []checkIssue
	for _, req := range requirements {
		if !strings.Contains(lower, strings.ToLower(req)) {
			issues = append(issues, checkIssue{
				category: "must_have",
				what:     req,
				where:    "draft",
			})
		}
	}
	return issues
}

// checkRequiredSections verifies that every Required section from the Outline
// has a matching title in the draft (case-insensitive substring match).
func checkRequiredSections(draft string, sections []outline.Section) []checkIssue {
	lower := strings.ToLower(draft)
	var issues []checkIssue
	for _, sec := range sections {
		if !sec.Required {
			continue
		}
		if !strings.Contains(lower, strings.ToLower(sec.Title)) {
			issues = append(issues, checkIssue{
				category: "required_section",
				what:     sec.Title,
				where:    "draft",
			})
		}
	}
	return issues
}

// checkRequiredForms verifies that each form number in FormattingRules.RequiredForms
// is acknowledged in the draft (case-insensitive substring match).
func checkRequiredForms(draft string, rules *outline.FormattingRules) []checkIssue {
	if rules == nil {
		return nil
	}
	lower := strings.ToLower(draft)
	var issues []checkIssue
	for _, form := range rules.RequiredForms {
		if !strings.Contains(lower, strings.ToLower(form)) {
			issues = append(issues, checkIssue{
				category: "required_form",
				what:     form,
				where:    "draft",
			})
		}
	}
	return issues
}

// checkPageLimit estimates page count at 250 words/page and flags when the
// draft exceeds the solicitation's stated page limit. No check is performed
// when PageLimit is unspecified (Specified == false).
func checkPageLimit(draft string, rules *outline.FormattingRules) []checkIssue {
	if rules == nil || rules.PageLimit == nil || !rules.PageLimit.Specified {
		return nil
	}
	parts := strings.Fields(rules.PageLimit.Value)
	if len(parts) == 0 {
		return nil
	}
	limit, err := strconv.Atoi(parts[0])
	if err != nil || limit <= 0 {
		return nil
	}
	wordCount := len(strings.Fields(draft))
	// Ceiling division: a 251-word draft occupies 2 pages at 250 words/page.
	estimatedPages := (wordCount + 249) / 250
	if estimatedPages > limit {
		return []checkIssue{{
			category: "page_limit",
			what:     fmt.Sprintf("estimated %d pages exceeds %d-page limit", estimatedPages, limit),
			where:    "draft",
		}}
	}
	return nil
}
