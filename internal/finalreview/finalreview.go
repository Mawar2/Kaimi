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

// wordsPerPage is the conversion factor used to estimate page count from word count.
const wordsPerPage = 250

// Input holds everything the Final Review agent needs to do its job.
//
// Draft is the proposal text approved by the human reviewer. Opportunity is
// the source federal opportunity, used to verify deadline and context.
// Outline is optional; when nil, only deadline and must-have checks run.
type Input struct {
	// Draft is the human-approved proposal text. Must not be empty.
	Draft string

	// Opportunity is the federal opportunity this proposal responds to.
	// Must not be nil.
	Opportunity *opportunity.Opportunity

	// Outline is the structured outline produced by the Outline agent.
	// When nil, required_section, required_form, and page_limit checks are skipped.
	Outline *outline.Outline
}

// issue captures one failed check, ready to be serialised into AgentResult.Flags.
type issue struct {
	category string // check name: must_have, required_section, required_form, page_limit
	what     string // the specific requirement not met
	where    string // where it should appear (typically "draft")
}

// String returns a human-readable detail string for inclusion in a flag value.
func (i issue) String() string {
	return fmt.Sprintf("%s: %q not found in %s", i.category, i.what, i.where)
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
// Input validation failures (nil opportunity, empty draft) are returned as
// errors. Soft failures — like an expired deadline or a missing required
// section — are expressed through the AgentResult status and Flags so the
// Manager can route them appropriately without crashing the pipeline.
//
// The agent never calls any submission API. StatusReadyToSubmit in the
// returned Result is a signal for a human to act on.
func (a *Agent) Review(ctx context.Context, in Input) (*agent.Result, error) {
	// Validate required inputs at the system boundary.
	if in.Opportunity == nil {
		return nil, fmt.Errorf("final-review: opportunity must not be nil")
	}
	if in.Draft == "" {
		return nil, fmt.Errorf("final-review: draft must not be empty")
	}

	// Deadline failure is fatal — we cannot submit after the deadline.
	if err := checkDeadline(in.Opportunity); err != nil {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusFailed,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("proposal cannot be submitted: %v", err),
			CompletedAt: time.Now().UTC(),
		}, nil
	}

	// Collect all soft issues across all checks.
	var issues []issue

	issues = append(issues, checkMustHave(in.Draft, in.Opportunity.Requirements)...)

	if in.Outline != nil {
		issues = append(issues, checkRequiredSections(in.Draft, in.Outline)...)
		if in.Outline.FormattingRules != nil {
			issues = append(issues, checkRequiredForms(in.Draft, in.Outline.FormattingRules)...)
			issues = append(issues, checkPageLimit(in.Draft, in.Outline.FormattingRules)...)
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

// checkMustHave scans the draft for each keyword in requirements. Any keyword
// not found (case-insensitive) produces a must_have issue.
func checkMustHave(draft string, requirements []string) []issue {
	lower := strings.ToLower(draft)
	var issues []issue
	for _, req := range requirements {
		if !strings.Contains(lower, strings.ToLower(req)) {
			issues = append(issues, issue{
				category: "must_have",
				what:     req,
				where:    "draft",
			})
		}
	}
	return issues
}

// checkRequiredSections verifies every Required section in the outline has a
// matching title in the draft (case-insensitive substring match).
func checkRequiredSections(draft string, ol *outline.Outline) []issue {
	lower := strings.ToLower(draft)
	var issues []issue
	for _, sec := range ol.Sections {
		if !sec.Required {
			continue
		}
		if !strings.Contains(lower, strings.ToLower(sec.Title)) {
			issues = append(issues, issue{
				category: "required_section",
				what:     sec.Title,
				where:    "draft",
			})
		}
	}
	return issues
}

// checkRequiredForms confirms each form number in RequiredForms is mentioned
// in the draft (case-insensitive substring match).
func checkRequiredForms(draft string, rules *outline.FormattingRules) []issue {
	lower := strings.ToLower(draft)
	var issues []issue
	for _, form := range rules.RequiredForms {
		if !strings.Contains(lower, strings.ToLower(form)) {
			issues = append(issues, issue{
				category: "required_form",
				what:     form,
				where:    "draft",
			})
		}
	}
	return issues
}

// checkPageLimit estimates the draft's page count at wordsPerPage words/page
// and flags an issue when it exceeds the stated limit. When the solicitation
// did not specify a page limit (Specified=false), no check is performed.
func checkPageLimit(draft string, rules *outline.FormattingRules) []issue {
	if rules.PageLimit == nil || !rules.PageLimit.Specified {
		return nil
	}

	// Value is formatted as "<n> pages" by the Outline agent's extractFormattingRules.
	limit, err := parsePageLimit(rules.PageLimit.Value)
	if err != nil {
		// Malformed value — skip rather than crash.
		return nil
	}

	wordCount := len(strings.Fields(draft))
	estimatedPages := (wordCount + wordsPerPage - 1) / wordsPerPage // ceiling division

	if estimatedPages > limit {
		return []issue{{
			category: "page_limit",
			what:     fmt.Sprintf("estimated %d pages exceeds limit of %d", estimatedPages, limit),
			where:    "draft",
		}}
	}
	return nil
}

// parsePageLimit extracts the integer page count from a value like "25 pages".
func parsePageLimit(value string) (int, error) {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return 0, fmt.Errorf("empty page limit value")
	}
	return strconv.Atoi(parts[0])
}

// buildFlags serialises a slice of issues into the Flags map format.
// Always populates "issues_found"; adds "issue_N" keys for each issue.
func buildFlags(issues []issue) map[string]string {
	flags := map[string]string{
		"issues_found": strconv.Itoa(len(issues)),
	}
	for i, iss := range issues {
		flags[fmt.Sprintf("issue_%d", i+1)] = iss.String()
	}
	return flags
}
