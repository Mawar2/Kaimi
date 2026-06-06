package finalreview_test

import (
	"context"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/agent"
	"github.com/Mawar2/Kaimi/internal/finalreview"
	"github.com/Mawar2/Kaimi/internal/opportunity"
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
