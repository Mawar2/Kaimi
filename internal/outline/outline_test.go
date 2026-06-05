package outline_test

import (
	"context"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/agent"
	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/outline"
)

// cachedOpportunity returns a realistic, fully-populated Opportunity for use in tests.
// This is the "cached test fixture" — no live SAM.gov API calls are made.
func cachedOpportunity() *opportunity.Opportunity {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	return &opportunity.Opportunity{
		ID:               "CACHED-TEST-001",
		Title:            "IT Systems Design and Integration Services",
		SolicitationNum:  "SOL-2026-TEST-001",
		Agency:           "Department of Defense",
		Office:           "Office of the CIO",
		PostedDate:       now,
		ResponseDeadline: now.Add(30 * 24 * time.Hour),
		NAICSCode:        "541512",
		NAICSDescription: "Computer Systems Design Services",
		SetAsideCode:     "SBA",
		Description:      "Provide IT systems design and integration services for DoD CIO.",
		Type:             "Solicitation",
		URL:              "https://sam.gov/test/cached-001",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// TestOutlineAgent_HappyPath verifies the agent returns success for a valid Opportunity.
func TestOutlineAgent_HappyPath(t *testing.T) {
	a := outline.New()
	opp := cachedOpportunity()

	result, err := a.Run(context.Background(), opp)

	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	if result.Status != agent.StatusSuccess {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusSuccess)
	}
	if result.AgentName == "" {
		t.Error("AgentName must not be empty")
	}
	if result.Summary == "" {
		t.Error("Summary must not be empty on success")
	}
	if result.Error != "" {
		t.Errorf("Error must be empty on success, got %q", result.Error)
	}
	if result.IsSuccess() == false {
		t.Error("IsSuccess() must return true on StatusSuccess")
	}
}

// TestOutlineAgent_NilOpportunity verifies the agent returns failed when given nil input.
func TestOutlineAgent_NilOpportunity(t *testing.T) {
	a := outline.New()

	result, err := a.Run(context.Background(), nil)

	if err == nil {
		t.Fatal("Run() should return a non-nil error when opportunity is nil")
	}
	if result == nil {
		t.Fatal("Run() should still return a Result even on failure")
	}
	if result.Status != agent.StatusFailed {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusFailed)
	}
	if result.Error == "" {
		t.Error("Error must be non-empty on failure")
	}
	if result.IsFailed() == false {
		t.Error("IsFailed() must return true on StatusFailed")
	}
}

// TestOutlineAgent_ContextCancelled verifies the agent respects context cancellation.
func TestOutlineAgent_ContextCancelled(t *testing.T) {
	a := outline.New()
	opp := cachedOpportunity()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	result, err := a.Run(ctx, opp)

	if err == nil {
		t.Fatal("Run() should return an error when context is cancelled")
	}
	if result == nil {
		t.Fatal("Run() should still return a Result on context cancellation")
	}
	if result.Status != agent.StatusFailed {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusFailed)
	}
}

// TestOutlineAgent_ResultFields verifies the result is correctly populated.
func TestOutlineAgent_ResultFields(t *testing.T) {
	a := outline.New()
	opp := cachedOpportunity()

	result, err := a.Run(context.Background(), opp)

	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}
	if result.NoticeID != opp.ID {
		t.Errorf("NoticeID = %q, want %q", result.NoticeID, opp.ID)
	}
	if result.CompletedAt.IsZero() {
		t.Error("CompletedAt must be set")
	}
}
