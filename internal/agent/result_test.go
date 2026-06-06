package agent_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/agent"
)

// TestStatusValues verifies that all Status constants have the expected string values.
func TestStatusValues(t *testing.T) {
	tests := []struct {
		status agent.Status
		want   string
	}{
		{agent.StatusSuccess, "success"},
		{agent.StatusFailed, "failed"},
		{agent.StatusNeedsHuman, "needs_human"},
		{agent.StatusReadyToSubmit, "ready_to_submit"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("Status %q = %q, want %q", tt.status, string(tt.status), tt.want)
		}
	}
}

// TestAgentResultHelpers verifies IsSuccess, IsFailed, NeedsHuman, and IsTerminal
// return correct values for each status.
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
			isTerminal: false, // needs_human is NOT terminal — agent must re-run after intervention
		},
		{
			name:       "ready_to_submit",
			status:     agent.StatusReadyToSubmit,
			isSuccess:  false,
			isFailed:   false,
			needsHuman: false,
			isTerminal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := agent.AgentResult{Status: tt.status}
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

// TestAgentResultJSONRoundTrip verifies that a fully-populated AgentResult survives
// a marshal → unmarshal cycle with no data loss.
func TestAgentResultJSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	original := agent.AgentResult{
		AgentName:   "hunter",
		Status:      agent.StatusSuccess,
		NoticeID:    "SAM-12345",
		Summary:     "Found 42 opportunities matching NAICS 541511",
		OutputRef:   "queue/2026-06-05/batch-001.json",
		Flags:       map[string]string{"cached": "true", "source": "sam.gov"},
		CompletedAt: now,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var roundtripped agent.AgentResult
	if err := json.Unmarshal(data, &roundtripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if roundtripped.AgentName != original.AgentName {
		t.Errorf("AgentName = %q, want %q", roundtripped.AgentName, original.AgentName)
	}
	if roundtripped.Status != original.Status {
		t.Errorf("Status = %q, want %q", roundtripped.Status, original.Status)
	}
	if roundtripped.NoticeID != original.NoticeID {
		t.Errorf("NoticeID = %q, want %q", roundtripped.NoticeID, original.NoticeID)
	}
	if roundtripped.Summary != original.Summary {
		t.Errorf("Summary = %q, want %q", roundtripped.Summary, original.Summary)
	}
	if roundtripped.OutputRef != original.OutputRef {
		t.Errorf("OutputRef = %q, want %q", roundtripped.OutputRef, original.OutputRef)
	}
	if len(roundtripped.Flags) != len(original.Flags) {
		t.Errorf("Flags length = %d, want %d", len(roundtripped.Flags), len(original.Flags))
	}
	for k, v := range original.Flags {
		if roundtripped.Flags[k] != v {
			t.Errorf("Flags[%q] = %q, want %q", k, roundtripped.Flags[k], v)
		}
	}
	if !roundtripped.CompletedAt.Equal(original.CompletedAt) {
		t.Errorf("CompletedAt = %v, want %v", roundtripped.CompletedAt, original.CompletedAt)
	}
}

// TestAgentResultOptionalFieldsOmittedWhenEmpty verifies that optional fields with
// zero values are omitted from JSON output.
func TestAgentResultOptionalFieldsOmittedWhenEmpty(t *testing.T) {
	r := agent.AgentResult{
		AgentName:   "hunter",
		Status:      agent.StatusSuccess,
		CompletedAt: time.Now(),
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal into map failed: %v", err)
	}

	optionalFields := []string{"notice_id", "summary", "output_ref", "flags", "error"}
	for _, field := range optionalFields {
		if _, present := raw[field]; present {
			t.Errorf("optional field %q should be omitted when empty, but was present in JSON", field)
		}
	}
}

// TestAgentResultWithError verifies that error information is correctly serialized
// and that IsFailed returns true.
func TestAgentResultWithError(t *testing.T) {
	r := agent.AgentResult{
		AgentName:   "hunter",
		Status:      agent.StatusFailed,
		Error:       "SAM.gov API returned 429: rate limit exceeded",
		CompletedAt: time.Now(),
	}

	if !r.IsFailed() {
		t.Error("IsFailed() = false, want true for StatusFailed")
	}
	if !r.IsTerminal() {
		t.Error("IsTerminal() = false, want true for StatusFailed")
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal into map failed: %v", err)
	}

	if _, present := raw["error"]; !present {
		t.Error("error field should be present in JSON when non-empty")
	}
	if raw["error"] != r.Error {
		t.Errorf("error = %q, want %q", raw["error"], r.Error)
	}
}
