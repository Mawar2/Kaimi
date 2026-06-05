package agent_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Mawar2/multi-agent-system/internal/agent"
)

func TestAgentStatusValues(t *testing.T) {
	tests := []struct {
		status   agent.AgentStatus
		expected string
	}{
		{agent.AgentStatusSuccess, "success"},
		{agent.AgentStatusFailed, "failed"},
		{agent.AgentStatusNeedsHuman, "needs_human"},
		{agent.AgentStatusReadyToSubmit, "ready_to_submit"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("got %q, want %q", tt.status, tt.expected)
			}
		})
	}
}

func TestAgentStatusIsTerminal(t *testing.T) {
	tests := []struct {
		status   agent.AgentStatus
		terminal bool
	}{
		{agent.AgentStatusSuccess, true},
		{agent.AgentStatusFailed, true},
		{agent.AgentStatusReadyToSubmit, true},
		{agent.AgentStatusNeedsHuman, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.terminal {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.terminal)
			}
		})
	}
}

func TestAgentResultJSONRoundTrip(t *testing.T) {
	original := &agent.AgentResult{
		AgentName: "test-agent",
		Status:    agent.AgentStatusReadyToSubmit,
		NoticeID:  "issue-42",
		Summary:   "implemented feature and opened PR",
		OutputRef: "https://github.com/Mawar2/Kaimi/pull/42",
		Flags:     []string{"tdd_complete", "conventions_checked"},
		Error:     "",
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
	if len(decoded.Flags) != len(original.Flags) {
		t.Errorf("Flags length: got %d, want %d", len(decoded.Flags), len(original.Flags))
	}
	if decoded.Error != original.Error {
		t.Errorf("Error: got %q, want %q", decoded.Error, original.Error)
	}
}

func TestAgentResultErrorOmittedWhenEmpty(t *testing.T) {
	result := &agent.AgentResult{
		AgentName: "agent",
		Status:    agent.AgentStatusSuccess,
		NoticeID:  "1",
		Summary:   "done",
		OutputRef: "pr/1",
		Flags:     nil,
		Error:     "",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if _, ok := m["error"]; ok {
		t.Error("expected 'error' field to be omitted when empty, but it was present")
	}
}

func TestAgentResultWithError(t *testing.T) {
	result := &agent.AgentResult{
		AgentName: "failing-agent",
		Status:    agent.AgentStatusFailed,
		NoticeID:  "99",
		Summary:   "could not apply patch",
		OutputRef: "",
		Flags:     []string{"needs_escalation"},
		Error:     "git apply failed: conflict in main.go",
	}

	if result.AgentName != "failing-agent" {
		t.Errorf("AgentName: got %q, want %q", result.AgentName, "failing-agent")
	}
	if result.Status != agent.AgentStatusFailed {
		t.Errorf("Status: got %q, want %q", result.Status, agent.AgentStatusFailed)
	}
	if !result.Status.IsTerminal() {
		t.Error("AgentStatusFailed must be terminal")
	}
	if result.NoticeID != "99" {
		t.Errorf("NoticeID: got %q, want %q", result.NoticeID, "99")
	}
	if result.Summary != "could not apply patch" {
		t.Errorf("Summary: got %q, want %q", result.Summary, "could not apply patch")
	}
	if result.OutputRef != "" {
		t.Errorf("OutputRef: expected empty, got %q", result.OutputRef)
	}
	if len(result.Flags) != 1 || result.Flags[0] != "needs_escalation" {
		t.Errorf("Flags: got %v, want [needs_escalation]", result.Flags)
	}
	if result.Error == "" {
		t.Error("expected non-empty Error for failed result")
	}
}

func TestStubAgentImplementsInterface(t *testing.T) {
	// Compile-time check: StubAgent must satisfy Agent.
	var _ agent.Agent = &agent.StubAgent{}
}

func TestStubAgentRun(t *testing.T) {
	stub := agent.NewStubAgent("my-stub")

	if stub.Name() != "my-stub" {
		t.Errorf("Name: got %q, want %q", stub.Name(), "my-stub")
	}

	result, err := stub.Run(context.Background(), "issue-8")
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Run: result is nil")
	}
	if result.AgentName != "my-stub" {
		t.Errorf("AgentName: got %q, want %q", result.AgentName, "my-stub")
	}
	if result.Status != agent.AgentStatusSuccess {
		t.Errorf("Status: got %q, want %q", result.Status, agent.AgentStatusSuccess)
	}
	if result.NoticeID != "issue-8" {
		t.Errorf("NoticeID: got %q, want %q", result.NoticeID, "issue-8")
	}
	if result.Summary == "" {
		t.Error("Summary must not be empty")
	}
}
