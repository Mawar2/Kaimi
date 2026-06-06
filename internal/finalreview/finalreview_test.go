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
// It contains "Executive Summary", "Technical Approach", "Past Performance",
// and the word "modernization" — but NOT "cybersecurity", "Management Approach",
// "Price/Cost Volume", or any government form numbers.
const draftFixture = `
# Technical Proposal — Enterprise IT Modernization

## Executive Summary
BlueMeta Technologies brings proven expertise in federal IT modernization...

## Technical Approach
Our approach follows a phased migration strategy...

## Past Performance
BlueMeta has successfully delivered similar engagements for DoD and civilian agencies...
`

// longDraft returns a draft string containing exactly n space-separated words.
func longDraft(words int) string {
	parts := make([]string, words)
	for i := range parts {
		parts[i] = "word"
	}
	return strings.Join(parts, " ")
}

// outlineWithSections returns an Outline containing the given sections and no
// formatting requirements.
func outlineWithSections(sections []outline.Section) *outline.Outline {
	return &outline.Outline{
		OpportunityID: "opp-fixture-001",
		Sections:      sections,
		FormattingRules: &outline.FormattingRules{
			PageLimit: &outline.FormattingRule{Specified: false},
		},
	}
}

// outlineWithForms returns an Outline with the given required forms and no
// required sections or page limit.
func outlineWithForms(forms []string) *outline.Outline {
	return &outline.Outline{
		OpportunityID: "opp-fixture-001",
		FormattingRules: &outline.FormattingRules{
			PageLimit:     &outline.FormattingRule{Specified: false},
			RequiredForms: forms,
		},
	}
}

// outlineWithPageLimit returns an Outline with the given page limit and no
// required sections or forms.
func outlineWithPageLimit(value string, specified bool) *outline.Outline {
	return &outline.Outline{
		OpportunityID: "opp-fixture-001",
		FormattingRules: &outline.FormattingRules{
			PageLimit: &outline.FormattingRule{Value: value, Specified: specified},
		},
	}
}

// ── Skeleton tests (retained from KAI-6) ─────────────────────────────────────

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
	// No requirements, no outline → all checks pass.
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q; summary: %s", result.Status, agent.StatusReadyToSubmit, result.Summary)
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
	// Documents the invariant: Final Review never triggers submission.
	// StatusReadyToSubmit is a signal for a human — not an automatic action.
	ctx := context.Background()
	a := finalreview.New()

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}

	_ = result.Status // human reads this and decides — no submission API is called
}

func TestReview_ExpiredDeadline_NotReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
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

// ── KAI-7 tests: must_have check ─────────────────────────────────────────────

func TestReview_MissingMustHave_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{"cybersecurity"} // not in draftFixture

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q; summary: %s", result.Status, agent.StatusNeedsHuman, result.Summary)
	}
}

func TestReview_MustHaveAddressed_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{"modernization"} // present in draftFixture

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q; summary: %s", result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}

func TestReview_MultipleMustHaves_AllMissing_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{"cybersecurity", "zero-trust"} // neither in draftFixture

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusNeedsHuman)
	}
	// Both missing requirements should be flagged.
	if result.Flags["issues_found"] != "2" {
		t.Errorf("issues_found = %q, want %q", result.Flags["issues_found"], "2")
	}
}

func TestReview_EmptyRequirements_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = nil // no must-haves

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusReadyToSubmit)
	}
}

// ── KAI-7 tests: required_section check ──────────────────────────────────────

func TestReview_MissingRequiredSection_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineWithSections([]outline.Section{
		{ID: "management_approach", Title: "Management Approach", Required: true},
	})

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture, // does not contain "Management Approach"
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q; summary: %s", result.Status, agent.StatusNeedsHuman, result.Summary)
	}
}

func TestReview_AllRequiredSectionsPresent_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineWithSections([]outline.Section{
		{ID: "executive_summary", Title: "Executive Summary", Required: true},
		{ID: "technical_approach", Title: "Technical Approach", Required: true},
		{ID: "past_performance", Title: "Past Performance", Required: true},
	})

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture, // contains all three titles
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q; summary: %s", result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}

func TestReview_OptionalSectionMissing_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineWithSections([]outline.Section{
		{ID: "management_approach", Title: "Management Approach", Required: false},
	})

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture, // does not contain "Management Approach"
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	// Required=false → section is optional and its absence is not an issue.
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q; summary: %s", result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}

func TestReview_NoOutline_SkipsOutlineChecks(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	// draftFixture is missing many standard sections, but with no Outline
	// provided the agent must skip section and form checks.
	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
		Outline:     nil,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q; summary: %s", result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}

// ── KAI-7 tests: required_form check ─────────────────────────────────────────

func TestReview_MissingRequiredForm_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineWithForms([]string{"SF-330"})

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture, // does not mention "SF-330"
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q; summary: %s", result.Status, agent.StatusNeedsHuman, result.Summary)
	}
}

func TestReview_RequiredFormMentioned_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineWithForms([]string{"SF-330"})
	draft := draftFixture + "\nPlease include the completed SF-330 form.\n"

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q; summary: %s", result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}

func TestReview_NoRequiredForms_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineWithForms(nil) // no required forms

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q; summary: %s", result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}

// ── KAI-7 tests: Flags reporting ─────────────────────────────────────────────

func TestReview_IssuesReportedInFlags(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{"cybersecurity"} // missing → one issue

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Flags == nil {
		t.Fatal("Flags is nil, want map with issues_found")
	}
	if result.Flags["issues_found"] != "1" {
		t.Errorf("issues_found = %q, want %q", result.Flags["issues_found"], "1")
	}
	if result.Flags["issue_1"] == "" {
		t.Error("issue_1 is empty, want a non-empty issue description")
	}
}

func TestReview_CleanDraft_FlagsHaveZeroIssues(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(), // no requirements
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Flags == nil {
		t.Fatal("Flags is nil, want map with issues_found")
	}
	if result.Flags["issues_found"] != "0" {
		t.Errorf("issues_found = %q, want %q", result.Flags["issues_found"], "0")
	}
	if _, ok := result.Flags["issue_1"]; ok {
		t.Errorf("issue_1 key present on clean result, want absent")
	}
}

func TestReview_IssueDetails_ContainWhatAndWhere(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{"cybersecurity"}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	issue := result.Flags["issue_1"]
	if !strings.Contains(issue, "cybersecurity") {
		t.Errorf("issue_1 = %q, want it to contain %q (the what)", issue, "cybersecurity")
	}
	if !strings.Contains(issue, "draft") {
		t.Errorf("issue_1 = %q, want it to contain %q (the where)", issue, "draft")
	}
}

// ── KAI-7 tests: page_limit check ────────────────────────────────────────────

func TestReview_DraftWithinPageLimit_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	// draftFixture is ~45 words → ceil(45/250)=1 page; limit=1 → within limit.
	ol := outlineWithPageLimit("1 pages", true)

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q; summary: %s", result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}

func TestReview_DraftExceedsPageLimit_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	// 251 words → ceil(251/250)=2 pages; limit=1 → exceeds.
	ol := outlineWithPageLimit("1 pages", true)

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       longDraft(251),
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q; summary: %s", result.Status, agent.StatusNeedsHuman, result.Summary)
	}
}

func TestReview_UnspecifiedPageLimit_NoIssue(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	// Specified=false → solicitation is silent; no page check should run.
	ol := outlineWithPageLimit("", false)

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       longDraft(10000), // far over any typical limit
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q; summary: %s", result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}
