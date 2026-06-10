package finalreview

import "testing"

func TestParseComplianceResponse_PlainJSON(t *testing.T) {
	raw := `{"findings":[{"requirement":"r1","source":"L","addressed":false,"note":"n"}]}`
	findings, err := parseComplianceResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Requirement != "r1" || findings[0].Addressed {
		t.Errorf("unexpected findings: %+v", findings)
	}
}

func TestParseComplianceResponse_FencedJSON(t *testing.T) {
	raw := "```json\n{\"findings\":[{\"requirement\":\"r1\",\"source\":\"M\",\"addressed\":true}]}\n```"
	findings, err := parseComplianceResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || !findings[0].Addressed {
		t.Errorf("fence not stripped / parsed wrong: %+v", findings)
	}
}

func TestParseComplianceResponse_Empty_Errors(t *testing.T) {
	if _, err := parseComplianceResponse("   "); err == nil {
		t.Error("expected error for empty response")
	}
}

func TestParseComplianceResponse_InvalidJSON_Errors(t *testing.T) {
	if _, err := parseComplianceResponse("not json"); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestStripCodeFence(t *testing.T) {
	cases := map[string]string{
		"```json\n{\"a\":1}\n```": `{"a":1}`,
		"```\n{\"a\":1}\n```":     `{"a":1}`,
		`{"a":1}`:                 `{"a":1}`,
	}
	for in, want := range cases {
		if got := stripCodeFence(in); got != want {
			t.Errorf("stripCodeFence(%q) = %q, want %q", in, got, want)
		}
	}
}
