package outline

import (
	"context"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/agent"
	"github.com/Mawar2/Kaimi/internal/opportunity"
)

// cachedOpportunity returns a realistic Opportunity for use in tests.
// This is the "cached test fixture" — no live API calls needed.
func cachedOpportunity() *opportunity.Opportunity {
	now := time.Now().UTC().Truncate(time.Second)
	return &opportunity.Opportunity{
		ID:               "CACHED-TEST-001",
		Title:            "IT Systems Design Services",
		SolicitationNum:  "SOL-2026-TEST-001",
		Agency:           "Department of Defense",
		Office:           "Office of the CIO",
		PostedDate:       now,
		ResponseDeadline: now.Add(30 * 24 * time.Hour),
		NAICSCode:        "541512",
		NAICSDescription: "Computer Systems Design Services",
		SetAsideCode:     "SBA",
		Description:      "Provide IT systems design and integration services.",
		Type:             "Solicitation",
		URL:              "https://sam.gov/test/cached-001",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// TestOutlineAgent_HappyPath verifies the agent returns success when given a valid Opportunity.
func TestOutlineAgent_HappyPath(t *testing.T) {
	ctx := context.Background()
	a := New()

	result, err := a.Run(ctx, cachedOpportunity())

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
		t.Error("Summary must not be empty")
	}
}

// TestOutlineAgent_NilOpportunity verifies the agent returns failed when given nil input.
func TestOutlineAgent_NilOpportunity(t *testing.T) {
	ctx := context.Background()
	a := New()

	result, err := a.Run(ctx, nil)

	if err == nil {
		t.Fatal("Run() should return an error when opportunity is nil")
	}
	if result == nil {
		t.Fatal("Run() should still return a Result even on failure")
	}
	if result.Status != agent.StatusFailed {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusFailed)
	}
}
