package outline

import (
	"testing"

	"github.com/Mawar2/Kaimi/internal/opportunity"
)

// sectionTitles extracts the title of each section for easy assertion.
func sectionTitles(sections []Section) []string {
	titles := make([]string, len(sections))
	for i, s := range sections {
		titles[i] = s.Title
	}
	return titles
}

// containsTitle reports whether title appears in the slice.
func containsTitle(titles []string, title string) bool {
	for _, t := range titles {
		if t == title {
			return true
		}
	}
	return false
}

// TestOutlineAgent_HappyPath verifies that a valid opportunity returns a non-nil
// Outline and a success Result.
func TestOutlineAgent_HappyPath(t *testing.T) {
	a := NewAgent()
	opp := &opportunity.Opportunity{
		ID:    "TEST-001",
		Title: "IT Support Services",
	}

	outline, result, err := a.Run(opp)

	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Run returned nil result")
	}
	if result.Status != "success" {
		t.Errorf("expected Status %q, got %q", "success", result.Status)
	}
	if outline == nil {
		t.Fatal("Run returned nil Outline")
	}
	if outline.OpportunityID != opp.ID {
		t.Errorf("expected OpportunityID %q, got %q", opp.ID, outline.OpportunityID)
	}
	if len(outline.Sections) == 0 {
		t.Error("expected at least one section in Outline")
	}
	if outline.GeneratedAt.IsZero() {
		t.Error("expected GeneratedAt to be set")
	}
}

// TestOutlineAgent_NilOpportunity verifies that a nil opportunity returns a failed
// Result and a nil Outline (not a panic or error).
func TestOutlineAgent_NilOpportunity(t *testing.T) {
	a := NewAgent()

	outline, result, err := a.Run(nil)

	if err != nil {
		t.Fatalf("Run returned unexpected error for nil input: %v", err)
	}
	if result == nil {
		t.Fatal("Run returned nil result")
	}
	if result.Status != "failed" {
		t.Errorf("expected Status %q for nil input, got %q", "failed", result.Status)
	}
	if outline != nil {
		t.Errorf("expected nil Outline for nil input, got %+v", outline)
	}
}

// TestBuildSections_BaseSections verifies that a sparse opportunity (no set-aside code
// and empty description) still produces all five required federal proposal volumes.
func TestBuildSections_BaseSections(t *testing.T) {
	opp := &opportunity.Opportunity{
		ID:    "TEST-002",
		Title: "Sparse Opportunity",
		// SetAsideCode and Description intentionally empty
	}

	sections := buildSections(opp)

	requiredBase := []string{
		"Executive Summary",
		"Technical Approach",
		"Management Approach",
		"Past Performance",
		"Price/Cost",
	}

	titles := sectionTitles(sections)
	for _, required := range requiredBase {
		if !containsTitle(titles, required) {
			t.Errorf("missing required base section %q in sparse opportunity", required)
		}
	}

	if len(sections) < len(requiredBase) {
		t.Errorf("expected at least %d sections, got %d", len(requiredBase), len(sections))
	}
}

// TestBuildSections_SetAside verifies that a non-empty SetAsideCode triggers the
// Subcontracting Plan section.
func TestBuildSections_SetAside(t *testing.T) {
	opp := &opportunity.Opportunity{
		ID:           "TEST-003",
		SetAsideCode: "SBA",
	}

	sections := buildSections(opp)
	titles := sectionTitles(sections)

	if !containsTitle(titles, "Subcontracting Plan") {
		t.Error("expected Subcontracting Plan when SetAsideCode is set")
	}
}

// TestBuildSections_NoSetAside verifies that an empty SetAsideCode does NOT trigger
// the Subcontracting Plan section.
func TestBuildSections_NoSetAside(t *testing.T) {
	opp := &opportunity.Opportunity{
		ID:           "TEST-004",
		SetAsideCode: "",
	}

	sections := buildSections(opp)
	titles := sectionTitles(sections)

	if containsTitle(titles, "Subcontracting Plan") {
		t.Error("did not expect Subcontracting Plan when SetAsideCode is empty")
	}
}

// TestBuildSections_KeywordsAddSections verifies that each description keyword triggers
// its corresponding conditional section.
func TestBuildSections_KeywordsAddSections(t *testing.T) {
	cases := []struct {
		description string
		keyword     string
		section     string
	}{
		{"key personnel required for this contract", "key personnel", "Key Personnel"},
		{"offeror must identify key staff members", "key staff", "Key Personnel"},
		{"quality assurance plan is required", "quality assurance", "Quality Assurance"},
		{"security clearance is required for all staff", "security", "Security Plan"},
		{"top secret clearance required", "clearance", "Security Plan"},
		{"transition plan for incumbent contractor", "transition", "Transition Plan"},
		{"phase-in period of 30 days required", "phase-in", "Transition Plan"},
		{"phase in period of 30 days required", "phase in", "Transition Plan"},
		{"data rights clause applies", "data rights", "Data Rights"},
		{"intellectual property provisions apply", "intellectual property", "Data Rights"},
	}

	for _, tc := range cases {
		t.Run(tc.keyword, func(t *testing.T) {
			opp := &opportunity.Opportunity{
				ID:          "TEST-005",
				Description: tc.description,
			}

			sections := buildSections(opp)
			titles := sectionTitles(sections)

			if !containsTitle(titles, tc.section) {
				t.Errorf("expected section %q for description containing %q", tc.section, tc.keyword)
			}
		})
	}
}

// TestBuildSections_SectionRationaleSet verifies that every generated section has a
// non-empty Rationale field — both base sections and conditional ones.
func TestBuildSections_SectionRationaleSet(t *testing.T) {
	opp := &opportunity.Opportunity{
		ID:           "TEST-006",
		SetAsideCode: "WOSB",
		Description:  "key personnel and quality assurance and security clearance and transition plan and data rights provisions apply",
	}

	sections := buildSections(opp)

	for _, s := range sections {
		if s.Rationale == "" {
			t.Errorf("section %q has an empty Rationale", s.Title)
		}
	}
}
