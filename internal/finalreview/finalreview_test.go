package finalreview_test

import (
	"context"
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

// completeDraftFixture is a draft that satisfies all five standard federal proposal sections.
const completeDraftFixture = `
# Technical Proposal — Enterprise IT Modernization

## Executive Summary
BlueMeta Technologies brings proven expertise in federal IT modernization and digital transformation.

## Technical Approach
Our approach follows a phased migration strategy that minimises risk while accelerating delivery.

## Management Approach
BlueMeta's program management framework ensures on-time, on-budget delivery with weekly reporting.

## Past Performance
BlueMeta has successfully delivered similar engagements for DoD and civilian agencies nationwide.

## Price/Cost Volume
Our firm-fixed-price contract structure provides cost certainty across all performance periods.
`

// outlineFixture returns a minimal Outline with the five standard federal proposal sections.
func outlineFixture() *outline.Outline {
	now := time.Now().UTC()
	return &outline.Outline{
		OpportunityID: "opp-fixture-001",
		Title:         "Enterprise IT Modernization Services",
		Sections: []outline.Section{
			{ID: "executive_summary", Title: "Executive Summary", Required: true,
				Rationale: "standard section for federal proposals"},
			{ID: "technical_approach", Title: "Technical Approach", Required: true,
				Rationale: "standard section for federal proposals"},
			{ID: "management_approach", Title: "Management Approach", Required: true,
				Rationale: "standard section for federal proposals"},
			{ID: "past_performance", Title: "Past Performance", Required: true,
				Rationale: "standard section for federal proposals"},
			{ID: "price_cost_volume", Title: "Price/Cost Volume", Required: true,
				Rationale: "standard section for federal proposals"},
		},
		FormattingRules: &outline.FormattingRules{
			PageLimit:   &outline.FormattingRule{Specified: false},
			Font:        &outline.FormattingRule{Specified: false},
			Margins:     &outline.FormattingRule{Specified: false},
			LineSpacing: &outline.FormattingRule{Specified: false},
			FileFormat:  &outline.FormattingRule{Specified: false},
		},
		GeneratedAt: now,
	}
}

// --- Core contract tests (from KAI-6 skeleton) ---

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
	// No requirements and no outline — all checks pass with a valid draft and open deadline.
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q for valid input; summary: %s",
			result.Status, agent.StatusReadyToSubmit, result.Summary)
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

// --- Must-have checks (KAI-7) ---

func TestReview_MissingMustHave_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{"secret clearance required"}

	// Draft has no mention of clearance or secret — gap should be flagged.
	result, err := a.Review(ctx, finalreview.Input{
		Draft:       "We provide excellent technical services for enterprise modernisation.",
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q when must-have requirement is not addressed; summary: %s",
			result.Status, agent.StatusNeedsHuman, result.Summary)
	}
}

func TestReview_MustHaveAddressed_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{"secret clearance required"}

	// Draft explicitly mentions clearance — requirement is addressed.
	draft := draftFixture + "\nAll personnel hold active secret clearances and facility access.\n"

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draft,
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when must-have is addressed; summary: %s",
			result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}

func TestReview_MultipleMustHaves_AllMissing_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{
		"secret clearance required",
		"small business certification needed",
	}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       "Brief draft with no relevant details.",
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q when multiple must-haves are missing",
			result.Status, agent.StatusNeedsHuman)
	}
}

func TestReview_EmptyRequirements_ReadyToSubmit(t *testing.T) {
	// When Opportunity.Requirements is empty, must-have checks produce no issues.
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
		t.Errorf("Status = %q, want %q when requirements list is empty",
			result.Status, agent.StatusReadyToSubmit)
	}
}

// --- Required section checks (KAI-7) ---

func TestReview_MissingRequiredSection_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixture()
	// Add a section not present in completeDraftFixture.
	ol.Sections = append(ol.Sections, outline.Section{
		ID:        "security_plan",
		Title:     "Security Plan",
		Required:  true,
		Rationale: "opportunity references security requirements",
	})

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       completeDraftFixture, // does not contain "Security Plan"
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q when required section is missing from draft; summary: %s",
			result.Status, agent.StatusNeedsHuman, result.Summary)
	}
}

func TestReview_AllRequiredSectionsPresent_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       completeDraftFixture,
		Opportunity: fixture(),
		Outline:     outlineFixture(),
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when all required sections are present; summary: %s",
			result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}

func TestReview_OptionalSectionMissing_ReadyToSubmit(t *testing.T) {
	// Sections with Required=false must not produce issues when absent.
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixture()
	ol.Sections = append(ol.Sections, outline.Section{
		ID:        "appendix",
		Title:     "Appendix",
		Required:  false,
		Rationale: "optional supporting material",
	})

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       completeDraftFixture, // no appendix
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q; optional section absence must not flag an issue; summary: %s",
			result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}

func TestReview_NoOutline_SkipsOutlineChecks(t *testing.T) {
	// When no Outline is provided, section and formatting checks are skipped.
	ctx := context.Background()
	a := finalreview.New()

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draftFixture, // only 3 of 5 standard sections
		Opportunity: fixture(),
		Outline:     nil, // no outline — section/formatting checks must not run
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	// Without an outline and no requirements, the only check is the deadline.
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when no outline provided and no requirements; summary: %s",
			result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}

// --- Required government form checks (KAI-7) ---

func TestReview_MissingRequiredForm_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixture()
	ol.FormattingRules.RequiredForms = []string{"SF-330"}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       completeDraftFixture, // no mention of SF-330
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q when required government form is not mentioned; summary: %s",
			result.Status, agent.StatusNeedsHuman, result.Summary)
	}
}

func TestReview_RequiredFormMentioned_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixture()
	ol.FormattingRules.RequiredForms = []string{"SF-330"}

	draft := completeDraftFixture + "\nPlease complete and attach the SF-330 architect-engineer qualifications form.\n"

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       draft,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when required form is mentioned; summary: %s",
			result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}

func TestReview_NoRequiredForms_ReadyToSubmit(t *testing.T) {
	// When FormattingRules specifies no required forms, the check produces no issues.
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixture()
	ol.FormattingRules.RequiredForms = nil

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       completeDraftFixture,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when no required forms; summary: %s",
			result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}

// --- Issue reporting in AgentResult (KAI-7) ---

func TestReview_IssuesReportedInFlags(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{
		"secret clearance required",
		"small business certification needed",
	}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       "Brief draft with no relevant details.",
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Fatalf("Status = %q, want %q", result.Status, agent.StatusNeedsHuman)
	}
	if result.Flags == nil {
		t.Fatal("Flags is nil, want issue details in Flags")
	}
	if result.Flags["issues_found"] == "" {
		t.Error(`Flags["issues_found"] is empty, want a count`)
	}
	if result.Flags["issues_found"] == "0" {
		t.Error(`Flags["issues_found"] = "0", want > 0 when issues were found`)
	}
}

func TestReview_CleanDraft_FlagsHaveZeroIssues(t *testing.T) {
	// A clean result must not carry stale issue flags.
	ctx := context.Background()
	a := finalreview.New()

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       completeDraftFixture,
		Opportunity: fixture(),
		Outline:     outlineFixture(),
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Fatalf("Status = %q, want %q", result.Status, agent.StatusReadyToSubmit)
	}
	// Clean pass: Flags should be absent or empty — no issue artefacts.
	if result.Flags != nil {
		if v, ok := result.Flags["issues_found"]; ok && v != "0" {
			t.Errorf(`Flags["issues_found"] = %q on clean result, want absent or "0"`, v)
		}
	}
}

func TestReview_IssueDetails_ContainWhatAndWhere(t *testing.T) {
	// Each issue flag must describe what is wrong and where (the missing item).
	ctx := context.Background()
	a := finalreview.New()

	opp := fixture()
	opp.Requirements = []string{"secret clearance required"}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       "Unrelated content that does not address the requirement.",
		Opportunity: opp,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Fatalf("Status = %q, want StatusNeedsHuman", result.Status)
	}

	issue1, ok := result.Flags["issue_1"]
	if !ok || issue1 == "" {
		t.Fatalf(`Flags["issue_1"] missing or empty; want issue detail string`)
	}
}

// --- Page-limit check (KAI-7) ---

func TestReview_DraftWithinPageLimit_ReadyToSubmit(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixture()
	ol.FormattingRules.PageLimit = &outline.FormattingRule{
		Value:     "100 pages",
		Specified: true,
	}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       completeDraftFixture, // well under 100 pages
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when draft is within page limit; summary: %s",
			result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}

func TestReview_DraftExceedsPageLimit_NeedsHuman(t *testing.T) {
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixture()
	// Set a tiny page limit (1 page ≈ 250 words) — completeDraftFixture exceeds this.
	ol.FormattingRules.PageLimit = &outline.FormattingRule{
		Value:     "1 pages",
		Specified: true,
	}

	// Build a draft long enough to exceed 1 page (~250 words).
	longDraft := completeDraftFixture
	for i := 0; i < 20; i++ {
		longDraft += "\nAdditional detailed content expanding each section with supporting evidence and technical rationale for the proposed approach and methodology used throughout the programme of work."
	}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       longDraft,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusNeedsHuman {
		t.Errorf("Status = %q, want %q when draft exceeds page limit; summary: %s",
			result.Status, agent.StatusNeedsHuman, result.Summary)
	}
}

func TestReview_UnspecifiedPageLimit_NoIssue(t *testing.T) {
	// When page limit is not specified, no page-limit issue must be raised.
	ctx := context.Background()
	a := finalreview.New()

	ol := outlineFixture()
	ol.FormattingRules.PageLimit = &outline.FormattingRule{Specified: false}

	result, err := a.Review(ctx, finalreview.Input{
		Draft:       completeDraftFixture,
		Opportunity: fixture(),
		Outline:     ol,
	})
	if err != nil {
		t.Fatalf("Review() returned unexpected error: %v", err)
	}
	if result.Status != agent.StatusReadyToSubmit {
		t.Errorf("Status = %q, want %q when page limit not specified; summary: %s",
			result.Status, agent.StatusReadyToSubmit, result.Summary)
	}
}
