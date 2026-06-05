package agent

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStatusValues(t *testing.T) {
	// Status string values are part of the serialized contract — changing them
	// breaks JSON stored in the queue.
	if StatusSuccess != "success" {
		t.Errorf("StatusSuccess = %q, want %q", StatusSuccess, "success")
	}
	if StatusFailed != "failed" {
		t.Errorf("StatusFailed = %q, want %q", StatusFailed, "failed")
	}
	if StatusNeedsHuman != "needs_human" {
		t.Errorf("StatusNeedsHuman = %q, want %q", StatusNeedsHuman, "needs_human")
	}
	if StatusReadyToSubmit != "ready_to_submit" {
		t.Errorf("StatusReadyToSubmit = %q, want %q", StatusReadyToSubmit, "ready_to_submit")
	}
}

func TestAgentResultHelpers(t *testing.T) {
	cases := []struct {
		status     Status
		isSuccess  bool
		isFailed   bool
		needsHuman bool
		isTerminal bool
	}{
		{StatusSuccess, true, false, false, true},
		{StatusFailed, false, true, false, true},
		{StatusNeedsHuman, false, false, true, false},
		{StatusReadyToSubmit, true, false, false, true},
	}

	for _, c := range cases {
		r := &AgentResult{Status: c.status}
		if r.IsSuccess() != c.isSuccess {
			t.Errorf("status=%s: IsSuccess()=%v want %v", c.status, r.IsSuccess(), c.isSuccess)
		}
		if r.IsFailed() != c.isFailed {
			t.Errorf("status=%s: IsFailed()=%v want %v", c.status, r.IsFailed(), c.isFailed)
		}
		if r.NeedsHuman() != c.needsHuman {
			t.Errorf("status=%s: NeedsHuman()=%v want %v", c.status, r.NeedsHuman(), c.needsHuman)
		}
		if r.IsTerminal() != c.isTerminal {
			t.Errorf("status=%s: IsTerminal()=%v want %v", c.status, r.IsTerminal(), c.isTerminal)
		}
	}
}

func TestAgentResultJSONRoundTrip(t *testing.T) {
	original := &AgentResult{
		AgentName:   "scorer",
		Status:      StatusSuccess,
		NoticeID:    "ABC-123-2026",
		Summary:     "Scored 87/100 - Strong NAICS match",
		OutputRef:   "opportunities/ABC-123-2026.json",
		Flags:       map[string]string{"score": "87", "recommendation": "BID"},
		CompletedAt: time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded AgentResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.AgentName != original.AgentName {
		t.Errorf("AgentName: got %q want %q", decoded.AgentName, original.AgentName)
	}
	if decoded.Status != original.Status {
		t.Errorf("Status: got %q want %q", decoded.Status, original.Status)
	}
	if decoded.NoticeID != original.NoticeID {
		t.Errorf("NoticeID: got %q want %q", decoded.NoticeID, original.NoticeID)
	}
	if decoded.Flags["score"] != "87" {
		t.Errorf("Flags[score]: got %q want %q", decoded.Flags["score"], "87")
	}
	if !decoded.CompletedAt.Equal(original.CompletedAt) {
		t.Errorf("CompletedAt: got %v want %v", decoded.CompletedAt, original.CompletedAt)
	}
}

func TestAgentResultOptionalFieldsOmittedWhenEmpty(t *testing.T) {
	// Only required fields populated — optional fields must not appear in JSON.
	result := &AgentResult{
		AgentName:   "hunter",
		Status:      StatusFailed,
		Error:       "SAM.gov API returned 429",
		CompletedAt: time.Now(),
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	for _, field := range []string{"notice_id", "summary", "output_ref", "flags"} {
		if _, present := m[field]; present {
			t.Errorf("field %q should be omitted when empty, but it appears in JSON", field)
		}
	}
}

func TestAgentResultWithError(t *testing.T) {
	result := &AgentResult{
		AgentName:   "hunter",
		Status:      StatusFailed,
		Error:       "SAM.gov API returned 429 (rate limit exceeded)",
		CompletedAt: time.Now(),
	}

	if result.Status != StatusFailed {
		t.Errorf("Status = %v, want failed", result.Status)
	}
	if !result.IsFailed() {
		t.Error("IsFailed() should be true")
	}
	if result.IsSuccess() {
		t.Error("IsSuccess() should be false")
	}
	if result.Error == "" {
		t.Error("Error field should not be empty for a failed result")
	}
	if !result.IsTerminal() {
		t.Error("IsTerminal() should be true for StatusFailed")
	}
}
