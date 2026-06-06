package agent_test

import (
	"context"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/agent"
)

// TestStubAgentRun verifies that StubAgent returns a successful AgentResult
// with required fields populated.
func TestStubAgentRun(t *testing.T) {
	stub := agent.NewStubAgent("test-stub")
	ctx := context.Background()

	result, err := stub.Run(ctx, "SAM-99999")
	if err != nil {
		t.Fatalf("StubAgent.Run returned unexpected error: %v", err)
	}

	if result.AgentName != "test-stub" {
		t.Errorf("AgentName = %q, want %q", result.AgentName, "test-stub")
	}
	if result.Status != agent.StatusSuccess {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusSuccess)
	}
	if result.NoticeID != "SAM-99999" {
		t.Errorf("NoticeID = %q, want %q", result.NoticeID, "SAM-99999")
	}
	if result.CompletedAt.IsZero() {
		t.Error("CompletedAt should not be zero")
	}
}

// TestStubAgentContextCancellation verifies that StubAgent respects context
// cancellation and returns an error when the context is cancelled.
func TestStubAgentContextCancellation(t *testing.T) {
	stub := agent.NewStubAgent("cancellation-test")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Small sleep to ensure the context expires before Run completes.
	time.Sleep(5 * time.Millisecond)

	_, err := stub.Run(ctx, "SAM-00001")
	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}
