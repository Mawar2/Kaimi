package dashboard

import "testing"

// TestRequirementAddressed locks the criteria matcher (issue #246 B6). The old
// gate check demanded the full requirement phrase verbatim, so a must-have the
// draft addressed in different words ("authorization" vs "authorized") was
// falsely reported missing. The matcher now scores significant-term overlap with
// light stemming. Cases cover verbatim, paraphrase, and genuinely-absent.
func TestRequirementAddressed(t *testing.T) {
	cases := []struct {
		name  string
		draft string // lowercased, as deriveCriteria passes it
		req   string
		want  bool
	}{
		{"verbatim", "we use fedramp high authorization controls", "FedRAMP High authorization", true},
		{"paraphrase stem", "deployed only fedramp high authorized tooling", "FedRAMP High authorization", true},
		{"absent", "a general cloud security posture", "FedRAMP High authorization", false},
		{"single term present", "fedramp high authorized tooling", "FedRAMP", true},
		{"single term absent", "fedramp high authorized tooling", "ISO 27001 certification", false},
		{"paraphrase modernize", "we will modernize the architecture", "architecture modernization", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := requirementAddressed(c.draft, c.req); got != c.want {
				t.Errorf("requirementAddressed(%q, %q) = %v, want %v", c.draft, c.req, got, c.want)
			}
		})
	}
}
