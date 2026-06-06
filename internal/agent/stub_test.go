package agent

import (
	"context"
	"testing"
)

// TestStubAgentRun verifies that StubAgent returns a successful AgentResult
// with all expected fields populated.
func TestStubAgentRun(t *testing.T) {
	a := &StubAgent{Name: "test-stub"}

	result, err := a.Run(context.Background(), "NOTICE-001")
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	if result.AgentName != "test-stub" {
		t.Errorf("AgentName = %q, want %q", result.AgentName, "test-stub")
	}
	if result.NoticeID != "NOTICE-001" {
		t.Errorf("NoticeID = %q, want %q", result.NoticeID, "NOTICE-001")
	}
	if !result.IsSuccess() {
		t.Errorf("Expected IsSuccess() to be true, got status %q", result.Status)
	}
	if result.CompletedAt.IsZero() {
		t.Error("CompletedAt should not be zero")
	}
}

// TestStubAgentContextCancellation verifies that StubAgent respects context
// cancellation and returns an error rather than a result.
func TestStubAgentContextCancellation(t *testing.T) {
	a := &StubAgent{Name: "test-stub"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before calling Run

	result, err := a.Run(ctx, "NOTICE-001")
	if err == nil {
		t.Error("Expected error when context is already cancelled")
	}
	if result != nil {
		t.Errorf("Expected nil result when context is cancelled, got %+v", result)
	}
}
