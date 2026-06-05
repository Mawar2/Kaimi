package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestStatus_Values verifies that each Status constant has the expected string value.
func TestStatus_Values(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusSuccess, "success"},
		{StatusFailed, "failed"},
		{StatusNeedsHuman, "needs_human"},
		{StatusReadyToSubmit, "ready_to_submit"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("Status value: got %q, want %q", string(tt.status), tt.want)
			}
		})
	}
}

// TestStatus_Helpers verifies the IsSuccess, IsFailed, NeedsHuman, and IsTerminal
// helper methods on the Status type.
func TestStatus_Helpers(t *testing.T) {
	tests := []struct {
		status     Status
		isSuccess  bool
		isFailed   bool
		needsHuman bool
		isTerminal bool
	}{
		{StatusSuccess, true, false, false, true},
		{StatusFailed, false, true, false, true},
		{StatusNeedsHuman, false, false, true, false},
		{StatusReadyToSubmit, false, false, false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if tt.status.IsSuccess() != tt.isSuccess {
				t.Errorf("IsSuccess(): got %v, want %v", tt.status.IsSuccess(), tt.isSuccess)
			}
			if tt.status.IsFailed() != tt.isFailed {
				t.Errorf("IsFailed(): got %v, want %v", tt.status.IsFailed(), tt.isFailed)
			}
			if tt.status.NeedsHuman() != tt.needsHuman {
				t.Errorf("NeedsHuman(): got %v, want %v", tt.status.NeedsHuman(), tt.needsHuman)
			}
			if tt.status.IsTerminal() != tt.isTerminal {
				t.Errorf("IsTerminal(): got %v, want %v", tt.status.IsTerminal(), tt.isTerminal)
			}
		})
	}
}

// TestAgentResult_JSONRoundTrip verifies that an AgentResult can be marshaled to
// JSON and back without losing any required fields.
func TestAgentResult_JSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	outputRef := "opportunity/opp-123"

	original := AgentResult{
		AgentName:   "hunter",
		Status:      StatusSuccess,
		Summary:     "Found 42 opportunities matching NAICS 541512.",
		OutputRef:   &outputRef,
		Flags:       map[string]string{"source": "sam.gov", "naics": "541512"},
		CompletedAt: now,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded AgentResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.AgentName != original.AgentName {
		t.Errorf("AgentName: got %q, want %q", decoded.AgentName, original.AgentName)
	}
	if decoded.Status != original.Status {
		t.Errorf("Status: got %q, want %q", decoded.Status, original.Status)
	}
	if decoded.Summary != original.Summary {
		t.Errorf("Summary: got %q, want %q", decoded.Summary, original.Summary)
	}
	if decoded.OutputRef == nil || *decoded.OutputRef != outputRef {
		t.Errorf("OutputRef: got %v, want %q", decoded.OutputRef, outputRef)
	}
	if decoded.Flags["source"] != "sam.gov" {
		t.Errorf("Flags[source]: got %q, want %q", decoded.Flags["source"], "sam.gov")
	}
	if !decoded.CompletedAt.Equal(original.CompletedAt) {
		t.Errorf("CompletedAt: got %v, want %v", decoded.CompletedAt, original.CompletedAt)
	}
}

// TestAgentResult_OptionalFieldsOmitted verifies that nil/zero optional fields
// are omitted from the JSON output (omitempty behavior).
func TestAgentResult_OptionalFieldsOmitted(t *testing.T) {
	result := AgentResult{
		AgentName:   "scorer",
		Status:      StatusFailed,
		Summary:     "Scoring failed: LLM returned invalid JSON.",
		CompletedAt: time.Now().UTC(),
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if _, ok := raw["output_ref"]; ok {
		t.Error("output_ref should be omitted when nil")
	}
	if _, ok := raw["flags"]; ok {
		t.Error("flags should be omitted when nil")
	}
}

// TestAgentResult_ErrorResult verifies an AgentResult carrying an error message
// in flags round-trips correctly.
func TestAgentResult_ErrorResult(t *testing.T) {
	result := AgentResult{
		AgentName:   "hunter",
		Status:      StatusFailed,
		Summary:     "SAM.gov API returned 429 Too Many Requests.",
		Flags:       map[string]string{"error_code": "429", "retry_after": "3600"},
		CompletedAt: time.Now().UTC(),
	}

	if !result.Status.IsFailed() {
		t.Error("Expected status to be failed")
	}
	if !result.Status.IsTerminal() {
		t.Error("Expected failed status to be terminal")
	}
	if result.Flags["error_code"] != "429" {
		t.Errorf("Flags[error_code]: got %q, want %q", result.Flags["error_code"], "429")
	}
}

// TestStubAgent_Execute verifies that StubAgent returns a valid AgentResult
// with the agent's configured name and status.
func TestStubAgent_Execute(t *testing.T) {
	stub := &StubAgent{Name: "test-agent"}
	ctx := context.Background()

	result, err := stub.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}

	if result.AgentName != "test-agent" {
		t.Errorf("AgentName: got %q, want %q", result.AgentName, "test-agent")
	}
	if result.Status != StatusSuccess {
		t.Errorf("Status: got %q, want %q", result.Status, StatusSuccess)
	}
	if result.Summary == "" {
		t.Error("Expected non-empty Summary")
	}
	if result.CompletedAt.IsZero() {
		t.Error("Expected non-zero CompletedAt")
	}
}

// TestStubAgent_ContextCancellation verifies that StubAgent respects context
// cancellation and returns an error when the context is already cancelled.
func TestStubAgent_ContextCancellation(t *testing.T) {
	stub := &StubAgent{Name: "cancellable-agent"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := stub.Execute(ctx)
	if err == nil {
		t.Error("Expected error when context is cancelled, got nil")
	}
}
