package agent_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/agent"
)

func TestStatusValues(t *testing.T) {
	tests := []struct {
		status   agent.Status
		expected string
	}{
		{agent.StatusSuccess, "success"},
		{agent.StatusFailed, "failed"},
		{agent.StatusNeedsHuman, "needs_human"},
		{agent.StatusReadyToSubmit, "ready_to_submit"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("got %q, want %q", tt.status, tt.expected)
			}
		})
	}
}

func TestAgentResultHelpers(t *testing.T) {
	tests := []struct {
		name       string
		status     agent.Status
		isSuccess  bool
		isFailed   bool
		needsHuman bool
		isTerminal bool
	}{
		{
			name:       "success",
			status:     agent.StatusSuccess,
			isSuccess:  true,
			isFailed:   false,
			needsHuman: false,
			isTerminal: true,
		},
		{
			name:       "failed",
			status:     agent.StatusFailed,
			isSuccess:  false,
			isFailed:   true,
			needsHuman: false,
			isTerminal: true,
		},
		{
			name:       "needs_human",
			status:     agent.StatusNeedsHuman,
			isSuccess:  false,
			isFailed:   false,
			needsHuman: true,
			isTerminal: false,
		},
		{
			name:       "ready_to_submit",
			status:     agent.StatusReadyToSubmit,
			isSuccess:  true,
			isFailed:   false,
			needsHuman: false,
			isTerminal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &agent.AgentResult{Status: tt.status, CompletedAt: time.Now()}
			if got := r.IsSuccess(); got != tt.isSuccess {
				t.Errorf("IsSuccess() = %v, want %v", got, tt.isSuccess)
			}
			if got := r.IsFailed(); got != tt.isFailed {
				t.Errorf("IsFailed() = %v, want %v", got, tt.isFailed)
			}
			if got := r.NeedsHuman(); got != tt.needsHuman {
				t.Errorf("NeedsHuman() = %v, want %v", got, tt.needsHuman)
			}
			if got := r.IsTerminal(); got != tt.isTerminal {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.isTerminal)
			}
		})
	}
}

func TestAgentResultJSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := &agent.AgentResult{
		AgentName:   "scorer",
		Status:      agent.StatusSuccess,
		NoticeID:    "ABC-123-2026",
		Summary:     "Scored 87/100 — strong match",
		OutputRef:   "opportunities/ABC-123-2026.json",
		Flags:       map[string]string{"score": "87", "recommendation": "BID"},
		CompletedAt: now,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded agent.AgentResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if decoded.AgentName != original.AgentName {
		t.Errorf("AgentName: got %q, want %q", decoded.AgentName, original.AgentName)
	}
	if decoded.Status != original.Status {
		t.Errorf("Status: got %q, want %q", decoded.Status, original.Status)
	}
	if decoded.NoticeID != original.NoticeID {
		t.Errorf("NoticeID: got %q, want %q", decoded.NoticeID, original.NoticeID)
	}
	if decoded.Summary != original.Summary {
		t.Errorf("Summary: got %q, want %q", decoded.Summary, original.Summary)
	}
	if decoded.OutputRef != original.OutputRef {
		t.Errorf("OutputRef: got %q, want %q", decoded.OutputRef, original.OutputRef)
	}
	if decoded.Flags["score"] != "87" {
		t.Errorf("Flags[score]: got %q, want %q", decoded.Flags["score"], "87")
	}
	if !decoded.CompletedAt.Equal(original.CompletedAt) {
		t.Errorf("CompletedAt: got %v, want %v", decoded.CompletedAt, original.CompletedAt)
	}
}

func TestAgentResultOptionalFieldsOmittedWhenEmpty(t *testing.T) {
	result := &agent.AgentResult{
		AgentName:   "hunter",
		Status:      agent.StatusSuccess,
		CompletedAt: time.Now(),
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	for _, field := range []string{"notice_id", "summary", "output_ref", "flags", "error"} {
		if _, ok := m[field]; ok {
			t.Errorf("expected %q to be omitted when empty, but it was present", field)
		}
	}
}

func TestAgentResultWithError(t *testing.T) {
	result := &agent.AgentResult{
		AgentName:   "hunter",
		Status:      agent.StatusFailed,
		Error:       "SAM.gov API returned 429 (rate limit exceeded)",
		CompletedAt: time.Now(),
	}

	if !result.IsFailed() {
		t.Error("IsFailed() should be true for StatusFailed")
	}
	if result.IsSuccess() {
		t.Error("IsSuccess() should be false for StatusFailed")
	}
	if result.Error == "" {
		t.Error("Error field must not be empty for a failed result")
	}
}

func TestStubAgentRun(t *testing.T) {
	stub := agent.NewStubAgent("test-stub")

	result, err := stub.Execute(context.Background(), "NOTICE-001")
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute: result is nil")
	}
	if result.AgentName != "test-stub" {
		t.Errorf("AgentName: got %q, want %q", result.AgentName, "test-stub")
	}
	if result.Status != agent.StatusSuccess {
		t.Errorf("Status: got %q, want %q", result.Status, agent.StatusSuccess)
	}
	if result.NoticeID != "NOTICE-001" {
		t.Errorf("NoticeID: got %q, want %q", result.NoticeID, "NOTICE-001")
	}
	if result.Summary == "" {
		t.Error("Summary must not be empty on success")
	}
	if result.CompletedAt.IsZero() {
		t.Error("CompletedAt must be set")
	}
}

func TestStubAgentContextCancellation(t *testing.T) {
	stub := agent.NewStubAgent("cancel-stub")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	result, err := stub.Execute(ctx, "NOTICE-002")
	if err == nil {
		t.Fatal("Execute: expected error on cancelled context, got nil")
	}
	if result == nil {
		t.Fatal("Execute: result must not be nil even on cancellation")
	}
	if result.Status != agent.StatusFailed {
		t.Errorf("Status: got %q, want %q on cancelled context", result.Status, agent.StatusFailed)
	}
	if result.Error == "" {
		t.Error("Error must describe the cancellation")
	}
}
