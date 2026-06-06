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
// Draft is the human-approved proposal text. Opportunity is the source federal
// opportunity, used to verify the deadline and must-have requirements. Outline,
// when non-nil, provides the expected sections and formatting rules extracted by
// the Outline agent; without it, only deadline and must-have checks run.
type Input struct {
	// Draft is the human-approved proposal text. Must not be empty.
	Draft string

	// Opportunity is the federal opportunity this proposal responds to.
	// Must not be nil.
	Opportunity *opportunity.Opportunity

	// Outline is the structured outline produced by the Outline agent (KAI-4).
	// When non-nil, required-section and formatting-rule checks run against it.
	// When nil, those checks are skipped and only deadline and must-have checks run.
	Outline *outline.Outline
}

// issue is an internal record of one problem found during the review pass.
// Issues are collected during the check functions and then reported via
// AgentResult.Flags so the Manager can surface them for a human.
type issue struct {
	category string // "must_have", "required_section", "required_form", "page_limit"
	what     string // human-readable description of the problem
	where    string // the missing item or mismatched value
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

// Review runs the final automated checks on a human-approved proposal draft.
//
// Checks performed, in order:
//  1. Deadline — expired deadline is an unrecoverable failure (StatusFailed).
//  2. Must-haves — each entry in Opportunity.Requirements must be addressed by
//     at least one significant keyword in the draft.
//  3. Required sections — when Input.Outline is non-nil, every Section with
//     Required=true must appear as a substring in the draft (case-insensitive).
//  4. Required government forms — when an Outline is provided, each form listed
//     in FormattingRules.RequiredForms must be mentioned in the draft.
//  5. Page limit — when an Outline specifies a page limit, a word-count heuristic
//     (250 words ≈ 1 page) checks whether the draft exceeds it.
//
// Returns StatusReadyToSubmit only when all checks pass — never as an automatic
// submission trigger. Returns StatusNeedsHuman when issues are found; details
// appear in the returned Result.Flags under "issues_found" and "issue_N" keys.
//
// Returns a Go error only for invalid input (nil opportunity, empty draft).
// All soft failures are expressed via AgentResult.Status so the Manager can
// route them without crashing the pipeline.
func (a *Agent) Review(ctx context.Context, in Input) (*agent.Result, error) {
	// Validate required inputs at the system boundary.
	if in.Opportunity == nil {
		return nil, fmt.Errorf("final-review: opportunity must not be nil")
	}
	if in.Draft == "" {
		return nil, fmt.Errorf("final-review: draft must not be empty")
	}

	// Deadline check: an expired deadline is an unrecoverable hard failure.
	if err := checkDeadline(in.Opportunity); err != nil {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusFailed,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("proposal cannot be submitted: %v", err),
			CompletedAt: time.Now().UTC(),
		}, nil
	}

	var issues []issue

	// Must-have check: each requirement in the opportunity must be addressed.
	issues = append(issues, checkMustHaves(in.Draft, in.Opportunity.Requirements)...)

	// Outline-based checks run only when the Outline agent's output is provided.
	if in.Outline != nil {
		issues = append(issues, checkRequiredSections(in.Draft, in.Outline.Sections)...)
		if in.Outline.FormattingRules != nil {
			issues = append(issues, checkRequiredForms(in.Draft, in.Outline.FormattingRules.RequiredForms)...)
			pl := in.Outline.FormattingRules.PageLimit
			if pl != nil && pl.Specified {
				issues = append(issues, checkPageLimit(in.Draft, pl.Value)...)
			}
		}
	}

	if len(issues) > 0 {
		return buildNeedsHumanResult(in.Opportunity.ID, issues), nil
	}

	return &agent.Result{
		AgentName:   agentName,
		Status:      agent.StatusReadyToSubmit,
		NoticeID:    in.Opportunity.ID,
		Summary:     "all automated checks passed; proposal is ready for human submission",
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

// checkMustHaves scans the draft for evidence that each requirement is addressed.
//
// A requirement is considered addressed when at least one significant keyword
// from it (longer than 3 characters, not a common stop word) appears anywhere
// in the draft (case-insensitive). Returns one issue per unaddressed requirement.
func checkMustHaves(draft string, requirements []string) []issue {
	if len(requirements) == 0 {
		return nil
	}

	draftLower := strings.ToLower(draft)
	var issues []issue

	for _, req := range requirements {
		if !requirementAddressed(draftLower, req) {
			issues = append(issues, issue{
				category: "must_have",
				what:     "requirement not addressed in draft",
				where:    req,
			})
		}
	}
	return issues
}

// requirementAddressed reports whether draftLower contains at least one
// significant keyword extracted from requirement (case-insensitive).
func requirementAddressed(draftLower, requirement string) bool {
	reqLower := strings.ToLower(requirement)

	// Words too short or too generic to be meaningful signals.
	stopWords := map[string]bool{
		"the": true, "and": true, "for": true, "are": true, "has": true,
		"have": true, "must": true, "with": true, "that": true, "this": true,
		"from": true, "will": true, "our": true, "not": true, "all": true,
		"any": true, "can": true, "its": true,
	}

	for _, word := range strings.Fields(reqLower) {
		word = strings.Trim(word, ".,;:!?\"'()")
		if len(word) > 3 && !stopWords[word] && strings.Contains(draftLower, word) {
			return true
		}
	}
	return false
}

// checkRequiredSections verifies that each required section from the outline
// has a matching title somewhere in the draft (case-insensitive substring).
// Optional sections (Required=false) are ignored.
func checkRequiredSections(draft string, sections []outline.Section) []issue {
	if len(sections) == 0 {
		return nil
	}

	draftLower := strings.ToLower(draft)
	var issues []issue

	for _, section := range sections {
		if !section.Required {
			continue
		}
		if !strings.Contains(draftLower, strings.ToLower(section.Title)) {
			issues = append(issues, issue{
				category: "required_section",
				what:     "required section is missing from draft",
				where:    section.Title,
			})
		}
	}
	return issues
}

// checkRequiredForms verifies that each required government form number is
// acknowledged somewhere in the draft (case-insensitive).
func checkRequiredForms(draft string, forms []string) []issue {
	if len(forms) == 0 {
		return nil
	}

	draftUpper := strings.ToUpper(draft)
	var issues []issue

	for _, form := range forms {
		if !strings.Contains(draftUpper, strings.ToUpper(form)) {
			issues = append(issues, issue{
				category: "required_form",
				what:     "required government form not acknowledged in draft",
				where:    form,
			})
		}
	}
	return issues
}

// checkPageLimit estimates whether the draft exceeds the specified page limit.
// Uses 250 words per page as a heuristic. Only fires when the limit value can
// be parsed as an integer (e.g. "25 pages" → 25). Unparseable values are skipped.
func checkPageLimit(draft, pageLimitValue string) []issue {
	var maxPages int
	if _, err := fmt.Sscanf(pageLimitValue, "%d", &maxPages); err != nil || maxPages <= 0 {
		return nil // limit not parseable; skip rather than raise a false positive
	}

	words := len(strings.Fields(draft))
	// Ceiling division: partial pages count as a full page.
	estimatedPages := (words + 249) / 250

	if estimatedPages > maxPages {
		return []issue{{
			category: "page_limit",
			what: fmt.Sprintf("draft may exceed %d-page limit (estimated %d pages from word count)",
				maxPages, estimatedPages),
			where: "entire document",
		}}
	}
	return nil
}

// buildNeedsHumanResult constructs a StatusNeedsHuman AgentResult with full
// issue details encoded in Flags. Each issue is stored as "issue_N" with a
// human-readable description of category, what, and where. The total count
// is stored under "issues_found".
func buildNeedsHumanResult(noticeID string, issues []issue) *agent.Result {
	flags := make(map[string]string, len(issues)+1)
	flags["issues_found"] = strconv.Itoa(len(issues))
	for i, iss := range issues {
		flags[fmt.Sprintf("issue_%d", i+1)] = fmt.Sprintf("[%s] %s: %s",
			iss.category, iss.what, iss.where)
	}

	return &agent.Result{
		AgentName:   agentName,
		Status:      agent.StatusNeedsHuman,
		NoticeID:    noticeID,
		Summary:     fmt.Sprintf("%d issue(s) require human attention before submission", len(issues)),
		Flags:       flags,
		CompletedAt: time.Now().UTC(),
	}
}
