package finalreview

import (
	"context"
	"fmt"
	"regexp"
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
// Outline is optional; when nil, only deadline and must-have checks run.
type Input struct {
	// Draft is the human-approved proposal text. Must not be empty.
	Draft string

	// Opportunity is the federal opportunity this proposal responds to.
	// Must not be nil.
	Opportunity *opportunity.Opportunity

	// Outline is the structured outline produced by the Outline agent.
	// When nil, section and form checks are skipped — no breaking change for
	// callers that predate KAI-7.
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

// issue captures a single automated check finding.
type issue struct {
	category string // check that produced the finding, e.g. "must_have"
	what     string // description of what is missing or wrong
	where    string // location in the draft, e.g. "draft body"
}

func (i issue) String() string {
	return fmt.Sprintf("category: %s; missing: %q; location: %s", i.category, i.what, i.where)
}

// Review runs the final automated checks on an approved proposal draft.
//
// Checks performed:
//   - deadline: hard failure (StatusFailed) if response deadline has passed
//   - must_have: flags requirements from Opportunity.Requirements not found in draft
//   - required_section: flags Required sections from Outline absent from draft (if Outline non-nil)
//   - required_form: flags RequiredForms from Outline not acknowledged in draft (if Outline non-nil)
//   - page_limit: flags drafts that exceed the solicitation's stated page limit (if Outline non-nil)
//
// All non-deadline issues produce StatusNeedsHuman. StatusReadyToSubmit is
// returned only when every check passes. Review never calls any submission API.
//
// Review returns an error only for invalid input (nil Opportunity, empty Draft).
// Soft failures are expressed through AgentResult.Status so the Manager can
// route them without crashing the pipeline.
func (a *Agent) Review(ctx context.Context, in Input) (*agent.Result, error) {
	// Validate required inputs at the system boundary.
	if in.Opportunity == nil {
		return nil, fmt.Errorf("final-review: opportunity must not be nil")
	}
	if in.Draft == "" {
		return nil, fmt.Errorf("final-review: draft must not be empty")
	}

	// Hard failure: an expired deadline makes submission impossible.
	if err := checkDeadline(in.Opportunity); err != nil {
		return &agent.Result{
			AgentName:   agentName,
			Status:      agent.StatusFailed,
			NoticeID:    in.Opportunity.ID,
			Summary:     fmt.Sprintf("proposal cannot be submitted: %v", err),
			CompletedAt: time.Now().UTC(),
		}, nil
	}

	// Collect soft issues from each check.
	var issues []issue
	issues = append(issues, checkMustHave(in.Draft, in.Opportunity.Requirements)...)

	if in.Outline != nil {
		issues = append(issues, checkRequiredSections(in.Draft, in.Outline.Sections)...)
		if in.Outline.FormattingRules != nil {
			issues = append(issues, checkRequiredForms(in.Draft, in.Outline.FormattingRules.RequiredForms)...)
			issues = append(issues, checkPageLimit(in.Draft, in.Outline.FormattingRules.PageLimit)...)
		}
	}

	status := agent.StatusReadyToSubmit
	summary := "all automated checks passed; proposal is ready for human submission"
	if len(issues) > 0 {
		status = agent.StatusNeedsHuman
		summary = fmt.Sprintf("%d issue(s) found; proposal needs human review before submission", len(issues))
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

// buildFlags converts a slice of issues into the AgentResult.Flags map.
// issues_found holds the count; issue_N holds the detail string for each issue.
func buildFlags(issues []issue) map[string]string {
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

// checkMustHave reports an issue for each requirement keyword from
// Opportunity.Requirements that does not appear in the draft (case-insensitive).
func checkMustHave(draft string, requirements []string) []issue {
	lower := strings.ToLower(draft)
	var issues []issue
	for _, req := range requirements {
		if !strings.Contains(lower, strings.ToLower(req)) {
			issues = append(issues, issue{
				category: "must_have",
				what:     req,
				where:    "draft body",
			})
		}
	}
	return issues
}

// checkRequiredSections reports an issue for each Required section from the
// Outline whose Title does not appear as a substring in the draft (case-insensitive).
func checkRequiredSections(draft string, sections []outline.Section) []issue {
	lower := strings.ToLower(draft)
	var issues []issue
	for _, s := range sections {
		if !s.Required {
			continue
		}
		if !strings.Contains(lower, strings.ToLower(s.Title)) {
			issues = append(issues, issue{
				category: "required_section",
				what:     s.Title,
				where:    "draft body",
			})
		}
	}
	return issues
}

// checkRequiredForms reports an issue for each government form number from
// FormattingRules.RequiredForms that is not acknowledged in the draft (case-insensitive).
func checkRequiredForms(draft string, requiredForms []string) []issue {
	lower := strings.ToLower(draft)
	var issues []issue
	for _, form := range requiredForms {
		if !strings.Contains(lower, strings.ToLower(form)) {
			issues = append(issues, issue{
				category: "required_form",
				what:     form,
				where:    "draft body",
			})
		}
	}
	return issues
}

// pageLimitNumRE extracts the first integer from a page-limit value string.
var pageLimitNumRE = regexp.MustCompile(`\d+`)

// checkPageLimit reports an issue when the draft's estimated page count (at
// 250 words per page) exceeds the solicitation's stated page limit. When the
// limit is not specified (Specified=false), no check is performed.
func checkPageLimit(draft string, pageLimit *outline.FormattingRule) []issue {
	if pageLimit == nil || !pageLimit.Specified {
		return nil
	}
	m := pageLimitNumRE.FindString(pageLimit.Value)
	if m == "" {
		return nil
	}
	limit, err := strconv.Atoi(m)
	if err != nil || limit <= 0 {
		return nil
	}

	wordCount := len(strings.Fields(draft))
	estimatedPages := wordCount / 250
	if estimatedPages > limit {
		return []issue{{
			category: "page_limit",
			what:     fmt.Sprintf("estimated %d pages exceeds limit of %d", estimatedPages, limit),
			where:    "draft body",
		}}
	}
	return nil
}
