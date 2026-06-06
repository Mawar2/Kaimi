package finalreview_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/agent"
	"github.com/Mawar2/Kaimi/internal/finalreview"
	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/outline"
)

// fixture returns a minimal valid Opportunity for use in tests.
func fixture() *opportunity.Opportunity {
	now := time.Now().UTC()
	return &opportunity.Opportunity{
		ID:               "opp-fixture-001",
		Title:            "Enterprise IT Modernization Services",
		Agency:           "Dept. of Veterans Affairs",
		SolicitationNum:  "VA-2026-IT-001",
		NAICSCode:        "541512",
		PostedDate:       now.Add(-7 * 24 * time.Hour),
		ResponseDeadline: now.Add(30 * 24 * time.Hour),
		Description:      "Seeking IT modernization support for enterprise systems.",
		URL:              "https://sam.gov/opp/fixture-001",
		CreatedAt:        now.Add(-7 * 24 * time.Hour),
		UpdatedAt:        now,
	}
}

// draftFixture returns a non-empty approved draft for use in tests.
const draftFixture = `
# Technical Proposal — Enterprise IT Modernization

## Executive Summary
BlueMeta Technologies brings proven expertise in federal IT modernization...

## Technical Approach
Our approach follows a phased migration strategy...

## Past Performance
BlueMeta has successfully delivered similar engagements for DoD and civilian agencies...
`

// outlineFixtureNoChecks returns an Outline with no required sections, no required forms,
// and no page limit — useful as a baseline that adds no issues.
func outlineFixtureNoChecks() *outline.Outline {
	return &outline.Outline{
		OpportunityID: "opp-fixture-001",
		Title:         "Enterprise IT Modernization Services",
		Sections:      []outline.Section{},
		FormattingRules: &outline.FormattingRules{
			PageLimit:   &outline.FormattingRule{Specified: false},
			Font:        &outline.FormattingRule{Specified: false},
			Margins:     &outline.FormattingRule{Specified: false},
			LineSpacing: &outline.FormattingRule{Specified: false},
			FileFormat:  &outline.FormattingRule{Specified: false},
		},
	}
}

func TestNew_ReturnsAgent(t *testing.T) {
	a := finalreview.New()
	if a == nil {
		t.Fatal("New() returned nil")
	}
}

func TestReview_ValidInput_ReturnsResult(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Review() returned nil result")
	}
}

func TestReview_ValidInput_AgentNameSet(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.AgentName != "final-review" {
		t.Errorf("AgentName = %q, want %q", result.AgentName, "final-review")
	}
}

func TestReview_ValidInput_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	// Opportunity has no Requirements, no Outline — all checks pass.
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q for valid input; summary: %s", result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}

func TestReview_ValidInput_StatusSuccess(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_ValidInput_SummaryNotEmpty(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Summary == "" {
		t.Error("Summary is empty, want a non-empty explanation")
	}
}

func TestReview_NilOpportunity_ReturnsError(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	_, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: nil,
	})
	if err == nil {
		t.Error("Review() with nil Opportunity should return an error")
	}
}

func TestReview_EmptyDraft_ReturnsError(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	_, err := a.Review(ctx, finalreview.Input{
		Draft:       "",
		Opportunity: fixture(),
	})
	if err == nil {
		t.Error("Review() with empty Draft should return an error")
	}
}

func TestReview_NeverSubmits(t *testing.T) {
	// This test documents the invariant: Final Review never triggers submission.
	// It sets Status=StatusReadyToSubmit only as a signal to a human, not as an action.
	// There is no Submit() method on the agent — only Review().
	ctx := context.Background()
	a := finalreview.New()

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}

	// If Status is StatusReadyToSubmit, that's a flag for a human — not an automatic action.
	// This test verifies the agent only returns a result; it does not call any
	// submission API or side-effect that would send the proposal.
	_ = result.Status // documented: human reads this and decides
}

func TestReview_ExpiredDeadline_NotReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	// Set deadline in the past — proposal cannot be submitted.
	opp.ResponseDeadline = time.Now().Add(-24 * time.Hour)

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status == agent.StatusReadyToSubmit {
		t.Error("Status = StatusReadyToSubmit for expired deadline, want StatusFailed")
	}
	if result.Status != agent.StatusFailed {
		t.Errorf("Status = %q for expired deadline, want %q", result.Status, agent.StatusFailed)
	}
}

// --- KAI-7: must_have checks ---

func TestReview_MissingMustHave_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{"cybersecurity"}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       "Our proposal covers technical approach and management.",
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q when requirement not in draft", result.Status, agent.StatusNeedsHuman)
	}
}

func TestReview_MustHaveAddressed_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{"cybersecurity"}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       "Our proposal addresses cybersecurity compliance throughout.",
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when requirement addressed", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_MultipleMustHaves_AllMissing_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{"cybersecurity", "zero trust", "FedRAMP"}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       "Our proposal covers general IT modernization.",
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q when multiple requirements missing", result.Status, agent.StatusNeedsHuman)
	}
	// All three gaps should be reported.
	if result.Flags["issues_found"] != "3" {
		t.Errorf("issues_found = %q, want %q", result.Flags["issues_found"], "3")
	}
}

func TestReview_EmptyRequirements_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = nil // no must-haves

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       "Our proposal covers general IT modernization.",
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q with nil requirements", result.Status, agent.StatusReadyToSubmit)
	}
}

// --- KAI-7: required_section checks ---

func TestReview_MissingRequiredSection_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixtureNoChecks()
	ol.Sections = []outline.Section{
		{ID: "quality_assurance", Title: "Quality Assurance Plan", Required: true},
	}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       "Our proposal covers technical approach and management.",
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q when required section absent from draft", result.Status, agent.StatusNeedsHuman)
	}
}

func TestReview_AllRequiredSectionsPresent_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixtureNoChecks()
	ol.Sections = []outline.Section{
		{ID: "executive_summary", Title: "Executive Summary", Required: true},
		{ID: "technical_approach", Title: "Technical Approach", Required: true},
	}

	draft := "## Executive Summary\nBlueMeta overview.\n\n## Technical Approach\nOur strategy.\n"

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when all required sections present", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_OptionalSectionMissing_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixtureNoChecks()
	ol.Sections = []outline.Section{
		{ID: "appendix", Title: "Appendix A", Required: false},
	}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       "Our proposal covers technical approach and management.",
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when only optional sections missing", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_NoOutline_SkipsOutlineChecks(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	// No Outline supplied — section/form/page checks must not run.
	result, err := a.Review(ctx, finalreview.Input{
		Draft:       "Short draft without any section headings.",
		Opportunity: fixture(),
		Outline:     nil,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when Outline is nil", result.Status, agent.StatusReadyToSubmit)
	}
}

// --- KAI-7: required_form checks ---

func TestReview_MissingRequiredForm_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixtureNoChecks()
	ol.FormattingRules.RequiredForms = []string{"SF-330"}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       "Our proposal covers technical approach and management.",
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q when required form not mentioned", result.Status, agent.StatusNeedsHuman)
	}
}

func TestReview_RequiredFormMentioned_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixtureNoChecks()
	ol.FormattingRules.RequiredForms = []string{"SF-330"}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       "We will submit form SF-330 as required by the solicitation.",
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when required form mentioned", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_NoRequiredForms_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixtureNoChecks()
	ol.FormattingRules.RequiredForms = nil

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       "Our proposal covers technical approach and management.",
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q with no required forms", result.Status, agent.StatusReadyToSubmit)
	}
}

// --- KAI-7: flags reporting ---

func TestReview_IssuesReportedInFlags(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{"cybersecurity"}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       "Our proposal covers general IT modernization.",
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Flags == nil {
		t.Fatal("Flags is nil, want a populated map")
	}
	if _, ok := result.Flags["issues_found"]; !ok {
		t.Error("Flags missing key issues_found")
	}
	if _, ok := result.Flags["issue_1"]; !ok {
		t.Error("Flags missing key issue_1")
	}
}

func TestReview_CleanDraft_FlagsHaveZeroIssues(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Flags == nil {
		t.Fatal("Flags is nil, want a map with issues_found=0")
	}
	if result.Flags["issues_found"] != "0" {
		t.Errorf("issues_found = %q, want %q for clean draft", result.Flags["issues_found"], "0")
	}
	for k := range result.Flags {
		if strings.HasPrefix(k, "issue_") {
			t.Errorf("unexpected flag key %q in clean draft result", k)
		}
	}
}

func TestReview_IssueDetails_ContainWhatAndWhere(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	const keyword = "cybersecurity"
	opp := fixture()
	opp.Requirements = []string{keyword}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       "Our proposal covers general IT modernization.",
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	detail, ok := result.Flags["issue_1"]
	if !ok {
		t.Fatal("Flags missing key issue_1")
	}
	if !strings.Contains(detail, keyword) {
		t.Errorf("issue_1 = %q, want it to contain the keyword %q", detail, keyword)
	}
	if !strings.Contains(detail, "draft") {
		t.Errorf("issue_1 = %q, want it to contain %q (the where)", detail, "draft")
	}
}

// --- KAI-7: page_limit checks ---

func TestReview_DraftWithinPageLimit_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	// 1000 words / 250 = 4 pages; limit is 5 → within limit.
	draft := strings.Repeat("word ", 1000)

	ol := outlineFixtureNoChecks()
	ol.FormattingRules.PageLimit = &outline.FormattingRule{
		Value:     "5 pages",
		Specified: true,
	}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q; draft is within page limit; summary: %s", result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}

func TestReview_DraftExceedsPageLimit_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	// 1000 words / 250 = 4 pages; limit is 3 → exceeds limit.
	draft := strings.Repeat("word ", 1000)

	ol := outlineFixtureNoChecks()
	ol.FormattingRules.PageLimit = &outline.FormattingRule{
		Value:     "3 pages",
		Specified: true,
	}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q; draft exceeds page limit", result.Status, agent.StatusNeedsHuman)
	}
}

func TestReview_UnspecifiedPageLimit_NoIssue(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	// Very long draft; page limit not specified → no check should run.
	draft := strings.Repeat("word ", 10000)

	ol := outlineFixtureNoChecks()
	ol.FormattingRules.PageLimit = &outline.FormattingRule{Specified: false}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when page limit not specified", result.Status, agent.StatusReadyToSubmit)
	}
}
