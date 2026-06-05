package agent

import (
	"context"
	"testing"
)

func TestStubAgentRun(t *testing.T) {
	a := NewStubAgent("test-agent")
	ctx := context.Background()

	result, err := a.Execute(ctx, "TEST-123")
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if result.AgentName != "test-agent" {
		t.Errorf("AgentName = %q, want %q", result.AgentName, "test-agent")
	}
	if result.Status != StatusSuccess {
		t.Errorf("Status = %v, want success", result.Status)
	}
	if result.NoticeID != "TEST-123" {
		t.Errorf("NoticeID = %q, want TEST-123", result.NoticeID)
	}
	if result.Summary == "" {
		t.Error("Summary should not be empty for a successful result")
	}
	if result.OutputRef == "" {
		t.Error("OutputRef should not be empty for a successful result")
	}
	if result.Flags["stub"] != "true" {
		t.Errorf("Flags[stub] = %q, want %q", result.Flags["stub"], "true")
	}
	if result.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set")
	}
	if !result.IsSuccess() {
		t.Error("IsSuccess() should be true")
	}
	if result.IsFailed() {
		t.Error("IsFailed() should be false")
	}
	if result.NeedsHuman() {
		t.Error("NeedsHuman() should be false")
	}
	if !result.IsTerminal() {
		t.Error("IsTerminal() should be true for a successful result")
	}
}

func TestStubAgentContextCancellation(t *testing.T) {
	a := NewStubAgent("test-agent")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately before Execute

	result, err := a.Execute(ctx, "TEST-123")
	if err != context.Canceled {
		t.Errorf("Execute() error = %v, want context.Canceled", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("Status = %v, want failed", result.Status)
	}
	if result.Error == "" {
		t.Error("Error should not be empty for cancelled context")
	}
	if !result.IsFailed() {
		t.Error("IsFailed() should be true for cancelled execution")
	}
	if !result.IsTerminal() {
		t.Error("IsTerminal() should be true for StatusFailed")
	}
}
