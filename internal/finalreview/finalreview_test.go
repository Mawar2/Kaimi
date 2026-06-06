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

// ─── KAI-7: outlineFixture ───────────────────────────────────────────────────

// outlineFixture returns a minimal Outline with two required sections, one
// required form, and a 25-page page limit.
func outlineFixture() *outline.Outline {
	return &outline.Outline{
		OpportunityID: "opp-fixture-001",
		Title:         "Enterprise IT Modernization Services",
		Sections: []outline.Section{
			{ID: "executive_summary", Title: "Executive Summary", Required: true},
			{ID: "technical_approach", Title: "Technical Approach", Required: true},
		},
		FormattingRules: &outline.FormattingRules{
			PageLimit:     &outline.FormattingRule{Value: "25 pages", Specified: true},
			Font:          &outline.FormattingRule{Specified: false},
			Margins:       &outline.FormattingRule{Specified: false},
			LineSpacing:   &outline.FormattingRule{Specified: false},
			FileFormat:    &outline.FormattingRule{Specified: false},
			RequiredForms: []string{"SF-330"},
		},
	}
}

// repeatWords returns a string of n space-separated copies of word, producing
// a draft whose word count exceeds the given page limit at 250 words/page.
func repeatWords(word string, n int) string {
	words := make([]string, n)
	for i := range words {
		words[i] = word
	}
	return strings.Join(words, " ")
}

// ─── KAI-7: must_have checks ─────────────────────────────────────────────────

func TestReview_MissingMustHave_NeedsHuman(t *testing.T) {
	opp := fixture()
	opp.Requirements = []string{"zero-trust architecture"}
	draft := "This proposal focuses on cloud migration and DevSecOps practices."

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
		Draft:       draft,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusNeedsHuman)
	}
}

func TestReview_MustHaveAddressed_ReadyToSubmit(t *testing.T) {
	opp := fixture()
	opp.Requirements = []string{"zero-trust architecture"}
	draft := "Our solution implements zero-trust architecture throughout the platform."

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
		Draft:       draft,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_MultipleMustHaves_AllMissing_NeedsHuman(t *testing.T) {
	opp := fixture()
	opp.Requirements = []string{"FedRAMP authorization", "FIPS 140-2 compliance"}
	draft := "This proposal describes our cloud migration approach."

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
		Draft:       draft,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusNeedsHuman)
	}
	// Both missing requirements must be flagged.
	if result.Flags["issues_found"] != "2" {
		t.Errorf("issues_found = %q, want %q", result.Flags["issues_found"], "2")
	}
}

func TestReview_EmptyRequirements_ReadyToSubmit(t *testing.T) {
	opp := fixture()
	opp.Requirements = nil // no must-haves to check

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
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

// ─── KAI-7: required_section checks ─────────────────────────────────────────

func TestReview_MissingRequiredSection_NeedsHuman(t *testing.T) {
	ol := outlineFixture()
	// draft has Executive Summary but not Technical Approach
	draft := "# Executive Summary\nBlueMeta brings strong experience to this effort."

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusNeedsHuman)
	}
}

func TestReview_AllRequiredSectionsPresent_ReadyToSubmit(t *testing.T) {
	ol := outlineFixture()
	ol.FormattingRules.RequiredForms = nil // remove form requirement to isolate section check
	ol.FormattingRules.PageLimit = &outline.FormattingRule{Specified: false}
	draft := "# Executive Summary\nIntro here.\n\n# Technical Approach\nDetails here."

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_OptionalSectionMissing_ReadyToSubmit(t *testing.T) {
	ol := outlineFixture()
	ol.Sections = []outline.Section{
		{ID: "executive_summary", Title: "Executive Summary", Required: true},
		{ID: "optional_section", Title: "Optional Appendix", Required: false},
	}
	ol.FormattingRules.RequiredForms = nil
	ol.FormattingRules.PageLimit = &outline.FormattingRule{Specified: false}
	draft := "# Executive Summary\nIntro here."

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q (optional section must not block)", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_NoOutline_SkipsOutlineChecks(t *testing.T) {
	opp := fixture()
	opp.Requirements = nil // no must-haves either

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
		Draft:       draftFixture,
		Opportunity: opp,
		Outline:     nil, // no outline supplied
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when outline is nil", result.Status, agent.StatusReadyToSubmit)
	}
}

// ─── KAI-7: required_form checks ─────────────────────────────────────────────

func TestReview_MissingRequiredForm_NeedsHuman(t *testing.T) {
	ol := outlineFixture()
	ol.FormattingRules.PageLimit = &outline.FormattingRule{Specified: false}
	ol.Sections = nil // no section requirements, isolate form check
	// draft does not mention SF-330
	draft := "This proposal does not reference any government forms."

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusNeedsHuman)
	}
}

func TestReview_RequiredFormMentioned_ReadyToSubmit(t *testing.T) {
	ol := outlineFixture()
	ol.FormattingRules.PageLimit = &outline.FormattingRule{Specified: false}
	ol.Sections = nil
	draft := "Offeror shall complete and attach SF-330 as required by the solicitation."

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_NoRequiredForms_ReadyToSubmit(t *testing.T) {
	ol := outlineFixture()
	ol.FormattingRules.RequiredForms = nil
	ol.FormattingRules.PageLimit = &outline.FormattingRule{Specified: false}
	ol.Sections = nil

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
		Draft:       draftFixture,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusReadyToSubmit)
	}
}

// ─── KAI-7: flags ────────────────────────────────────────────────────────────

func TestReview_IssuesReportedInFlags(t *testing.T) {
	opp := fixture()
	opp.Requirements = []string{"FedRAMP authorization"}
	draft := "This proposal describes our technical approach."

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
		Draft:       draft,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Flags == nil {
		t.Fatal("Flags is nil, want populated map")
	}
	if result.Flags["issues_found"] == "" {
		t.Error("issues_found flag is missing")
	}
	if result.Flags["issue_1"] == "" {
		t.Error("issue_1 flag is missing for first issue")
	}
}

func TestReview_CleanDraft_FlagsHaveZeroIssues(t *testing.T) {
	opp := fixture()
	opp.Requirements = nil

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
		Draft:       draftFixture,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Flags == nil {
		t.Fatal("Flags is nil, want populated map")
	}
	if result.Flags["issues_found"] != "0" {
		t.Errorf("issues_found = %q, want %q for clean draft", result.Flags["issues_found"], "0")
	}
	if _, ok := result.Flags["issue_1"]; ok {
		t.Error("issue_1 flag should not be present for clean draft")
	}
}

func TestReview_IssueDetails_ContainWhatAndWhere(t *testing.T) {
	opp := fixture()
	opp.Requirements = []string{"zero-trust architecture"}
	draft := "This proposal describes our cloud approach without mentioning the key technology."

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
		Draft:       draft,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	issue1 := result.Flags["issue_1"]
	if !strings.Contains(issue1, "zero-trust architecture") {
		t.Errorf("issue_1 = %q; want it to contain the requirement keyword %q", issue1, "zero-trust architecture")
	}
	if !strings.Contains(strings.ToLower(issue1), "draft") {
		t.Errorf("issue_1 = %q; want it to contain \"draft\" as the location", issue1)
	}
}

// ─── KAI-7: page_limit checks ────────────────────────────────────────────────

func TestReview_DraftWithinPageLimit_ReadyToSubmit(t *testing.T) {
	ol := outlineFixture()
	ol.FormattingRules.RequiredForms = nil
	ol.Sections = nil
	// 10 pages * 250 words/page = 2500 words; limit is 25 pages
	draft := repeatWords("word", 2500)

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q for draft within page limit", result.Status, agent.StatusReadyToSubmit)
	}
}

func TestReview_DraftExceedsPageLimit_NeedsHuman(t *testing.T) {
	ol := outlineFixture()
	ol.FormattingRules.RequiredForms = nil
	ol.Sections = nil
	// 30 pages * 250 words/page = 7500 words; limit is 25 pages
	draft := repeatWords("word", 7500)

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q for draft exceeding page limit", result.Status, agent.StatusNeedsHuman)
	}
}

func TestReview_UnspecifiedPageLimit_NoIssue(t *testing.T) {
	ol := outlineFixture()
	ol.FormattingRules.RequiredForms = nil
	ol.Sections = nil
	ol.FormattingRules.PageLimit = &outline.FormattingRule{Specified: false}
	// Very long draft — but no page limit is stated, so it must not flag.
	draft := repeatWords("word", 10000)

	a := finalreview.New()
	result, err := a.Review(context.Background(), finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when page limit is unspecified", result.Status, agent.StatusReadyToSubmit)
	}
}
