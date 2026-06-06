package agent

import (
	"encoding/json"
	"testing"
	"time"
)

// TestStatusValues verifies that the Status enum constants have the correct string values.
func TestStatusValues(t *testing.T) {
	tests := []struct {
		name  string
		value Status
		want  string
	}{
		{"success", StatusSuccess, "success"},
		{"failed", StatusFailed, "failed"},
		{"needs_human", StatusNeedsHuman, "needs_human"},
		{"ready_to_submit", StatusReadyToSubmit, "ready_to_submit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.want {
				t.Errorf("Status %s = %q, want %q", tt.name, tt.value, tt.want)
			}
		})
	}
}

// TestAgentResultHelpers verifies IsSuccess, IsFailed, NeedsHuman, and IsTerminal
// for every status value.
func TestAgentResultHelpers(t *testing.T) {
	tests := []struct {
		name       string
		status     Status
		isSuccess  bool
		isFailed   bool
		needsHuman bool
		isTerminal bool
	}{
		{"success", StatusSuccess, true, false, false, true},
		{"failed", StatusFailed, false, true, false, true},
		{"needs_human", StatusNeedsHuman, false, false, true, false},
		{"ready_to_submit", StatusReadyToSubmit, false, false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &AgentResult{Status: tt.status}
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

// TestAgentResultJSONRoundTrip verifies that AgentResult serializes and
// deserializes correctly with all fields populated.
func TestAgentResultJSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := &AgentResult{
		AgentName:   "hunter",
		Status:      StatusSuccess,
		NoticeID:    "ABC123",
		Summary:     "found 42 opportunities",
		OutputRef:   "queue/hunter-run-001",
		Flags:       map[string]string{"naics": "541512", "count": "42"},
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
		t.Errorf("AgentName = %q, want %q", decoded.AgentName, original.AgentName)
	}
	if decoded.Status != original.Status {
		t.Errorf("Status = %q, want %q", decoded.Status, original.Status)
	}
	if decoded.NoticeID != original.NoticeID {
		t.Errorf("NoticeID = %q, want %q", decoded.NoticeID, original.NoticeID)
	}
	if decoded.Summary != original.Summary {
		t.Errorf("Summary = %q, want %q", decoded.Summary, original.Summary)
	}
	if decoded.OutputRef != original.OutputRef {
		t.Errorf("OutputRef = %q, want %q", decoded.OutputRef, original.OutputRef)
	}
	if decoded.Flags["naics"] != original.Flags["naics"] {
		t.Errorf("Flags[naics] = %q, want %q", decoded.Flags["naics"], original.Flags["naics"])
	}
	if !decoded.CompletedAt.Equal(original.CompletedAt) {
		t.Errorf("CompletedAt = %v, want %v", decoded.CompletedAt, original.CompletedAt)
	}
}

// TestAgentResultOptionalFieldsOmittedWhenEmpty verifies that output_ref, flags,
// and error are absent from JSON when empty/nil.
func TestAgentResultOptionalFieldsOmittedWhenEmpty(t *testing.T) {
	r := &AgentResult{
		AgentName:   "hunter",
		Status:      StatusSuccess,
		NoticeID:    "ABC123",
		Summary:     "done",
		CompletedAt: time.Now(),
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if _, ok := raw["output_ref"]; ok {
		t.Error("output_ref should be omitted when empty")
	}
	if _, ok := raw["flags"]; ok {
		t.Error("flags should be omitted when nil")
	}
	if _, ok := raw["error"]; ok {
		t.Error("error should be omitted when empty")
	}
}

// TestAgentResultWithError verifies that error information is correctly stored
// and included in JSON serialization.
func TestAgentResultWithError(t *testing.T) {
	r := &AgentResult{
		AgentName:   "hunter",
		Status:      StatusFailed,
		NoticeID:    "ABC123",
		Summary:     "failed to fetch opportunities",
		Error:       "connection refused: sam.gov",
		CompletedAt: time.Now(),
	}

	if !r.IsFailed() {
		t.Error("Expected IsFailed() to be true")
	}
	if !r.IsTerminal() {
		t.Error("Expected IsTerminal() to be true for failed status")
	}
	if r.Error == "" {
		t.Error("Expected Error field to be set")
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if _, ok := raw["error"]; !ok {
		t.Error("error field should be present in JSON when set")
	}
}
