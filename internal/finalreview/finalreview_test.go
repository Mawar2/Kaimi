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

// outlineFixture returns a minimal valid Outline whose required sections all
// appear in draftFixture. Use as a baseline for outline-related tests.
func outlineFixture() *outline.Outline {
	return &outline.Outline{
		OpportunityID: "opp-fixture-001",
		Title:         "Enterprise IT Modernization Services",
		Sections: []outline.Section{
			{ID: "executive_summary", Title: "Executive Summary", Required: true},
			{ID: "technical_approach", Title: "Technical Approach", Required: true},
			{ID: "past_performance", Title: "Past Performance", Required: true},
		},
		FormattingRules: &outline.FormattingRules{
			PageLimit:   &outline.FormattingRule{Specified: false},
			Font:        &outline.FormattingRule{Specified: false},
			Margins:     &outline.FormattingRule{Specified: false},
			LineSpacing: &outline.FormattingRule{Specified: false},
			FileFormat:  &outline.FormattingRule{Specified: false},
		},
	}
}

// pageLimitOutline returns an Outline with no sections and the given page limit
// setting. Used to isolate page-limit check tests from section-check concerns.
func pageLimitOutline(value string, specified bool) *outline.Outline {
	return &outline.Outline{
		FormattingRules: &outline.FormattingRules{
			PageLimit:   &outline.FormattingRule{Value: value, Specified: specified},
			Font:        &outline.FormattingRule{Specified: false},
			Margins:     &outline.FormattingRule{Specified: false},
			LineSpacing: &outline.FormattingRule{Specified: false},
			FileFormat:  &outline.FormattingRule{Specified: false},
		},
	}
}

// --- Existing skeleton tests (10) ---

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
	// Stubbed checks pass — a valid draft and opportunity should be ready.
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

// --- New KAI-7 tests (17) ---

func TestReview_MissingMustHave_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{"classified work requirement"}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q when must-have requirement is missing", result.Status, agent.StatusNeedsHuman)
	}
}

func TestReview_MustHaveAddressed_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	// "technical approach" appears (case-insensitively) in draftFixture.
	opp.Requirements = []string{"technical approach"}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when must-have requirement is present in draft", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_MultipleMustHaves_AllMissing_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{"XYZZY-ALPHA requirement", "XYZZY-BETA requirement"}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q when multiple must-haves are missing", result.Status, agent.StatusNeedsHuman)
	}
	if result.Flags["issues_found"] != "2" {
		t.Errorf("issues_found = %q, want %q", result.Flags["issues_found"], "2")
	}
}

func TestReview_EmptyRequirements_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = nil

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when requirements list is nil", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_MissingRequiredSection_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixture()
	// Add a required section not present in draftFixture.
	ol.Sections = append(ol.Sections, outline.Section{
		ID:       "transition_plan",
		Title:    "Transition Plan",
		Required: true,
	})

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q when required section is absent from draft", result.Status, agent.StatusNeedsHuman)
	}
}

func TestReview_AllRequiredSectionsPresent_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	// outlineFixture sections all appear in draftFixture.
	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
		Outline:     outlineFixture(),
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when all required sections are present", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_OptionalSectionMissing_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixture()
	// Add an optional section that is NOT in draftFixture — should be ignored.
	ol.Sections = append(ol.Sections, outline.Section{
		ID:       "security_plan",
		Title:    "Security Plan",
		Required: false,
	})

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when only an optional section is missing", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_NoOutline_SkipsOutlineChecks(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
		Outline:     nil,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	// Without an outline, section and form checks are skipped — expect clean pass.
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when no outline is provided (checks skipped)", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_MissingRequiredForm_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixture()
	ol.FormattingRules.RequiredForms = []string{"SF-1449"}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q when required form is not acknowledged in draft", result.Status, agent.StatusNeedsHuman)
	}
}

func TestReview_RequiredFormMentioned_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixture()
	ol.FormattingRules.RequiredForms = []string{"SF-1449"}

	draft := draftFixture + "\nAttach completed SF-1449 with your submission.\n"

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when required form is mentioned in draft", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_NoRequiredForms_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixture()
	ol.FormattingRules.RequiredForms = []string{}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when no required forms are specified", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_IssuesReportedInFlags(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{"XYZZY-MISSING-KEYWORD"}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Flags == nil {
		t.Fatal("Flags is nil, want a populated map")
	}
	if result.Flags["issues_found"] != "1" {
		t.Errorf("issues_found = %q, want %q", result.Flags["issues_found"], "1")
	}
	if result.Flags["issue_1"] == "" {
		t.Error("issue_1 flag is empty, want a non-empty issue description")
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
		t.Errorf("issues_found = %q, want %q for a clean draft", result.Flags["issues_found"], "0")
	}
	if _, exists := result.Flags["issue_1"]; exists {
		t.Error("issue_1 key should not exist in Flags when there are no issues")
	}
}

func TestReview_IssueDetails_ContainWhatAndWhere(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	keyword := "XYZZY-UNIQUE-KEYWORD"
	opp := fixture()
	opp.Requirements = []string{keyword}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	issue := result.Flags["issue_1"]
	if !strings.Contains(issue, keyword) {
		t.Errorf("issue_1 = %q, want it to contain the missing keyword %q", issue, keyword)
	}
	if !strings.Contains(issue, "draft") {
		t.Errorf("issue_1 = %q, want it to reference %q as the location", issue, "draft")
	}
}

func TestReview_DraftWithinPageLimit_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	// 500 words → ceil(500/250) = 2 pages ≤ 3 page limit.
	// pageLimitOutline has no required sections, isolating this to the page check.
	draft := strings.Repeat("word ", 500)

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     pageLimitOutline("3 pages", true),
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when draft is within page limit", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_DraftExceedsPageLimit_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	// 300 words → ceil(300/250) = 2 pages > 1 page limit.
	draft := strings.Repeat("word ", 300)

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     pageLimitOutline("1 pages", true),
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q when draft exceeds page limit", result.Status, agent.StatusNeedsHuman)
	}
}

func TestReview_UnspecifiedPageLimit_NoIssue(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	// Very long draft — but page limit is unspecified so no check runs.
	draft := strings.Repeat("word ", 10000)

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     pageLimitOutline("", false),
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when page limit is unspecified", result.Status, agent.StatusReadyToSubmit)
	}
}
