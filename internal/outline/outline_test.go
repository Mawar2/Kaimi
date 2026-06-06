package outline

import (
	"context"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/agent"
	"github.com/Mawar2/Kaimi/internal/opportunity"
)

var testTime = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

// baseOpportunity returns a minimal but valid Opportunity for testing.
func baseOpportunity() *opportunity.Opportunity {
	return &opportunity.Opportunity{
		ID:               "TEST-001",
		Title:            "IT Systems Design Services",
		SolicitationNum:  "SOL-2026-TEST-001",
		Agency:           "Department of Defense",
		Office:           "Office of the CIO",
		PostedDate:       testTime,
		ResponseDeadline: testTime.Add(30 * 24 * time.Hour),
		NAICSCode:        "541512",
		NAICSDescription: "Computer Systems Design Services",
		SetAsideCode:     "",
		Description:      "Provide IT systems design and integration services.",
		Type:             "Solicitation",
		URL:              "https://sam.gov/test/001",
		CreatedAt:        testTime,
		UpdatedAt:        testTime,
	}
}

// TestOutlineAgent_HappyPath verifies the agent returns a valid Outline and success result.
func TestOutlineAgent_HappyPath(t *testing.T) {
	ctx := context.Background()
	a := New()

	outline, result, err := a.Run(ctx, baseOpportunity())

	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	if result.Status != agent.StatusSuccess {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusSuccess)
	}
	if result.AgentName != agentName {
		t.Errorf("AgentName = %q, want %q", result.AgentName, agentName)
	}
	const wantSummary = "generated 5 sections for opportunity TEST-001"
	if result.Summary != wantSummary {
		t.Errorf("Summary = %q, want %q", result.Summary, wantSummary)
	}
	if outline == nil {
		t.Fatal("Run() returned nil outline on success")
	}
	if outline.OpportunityID != "TEST-001" {
		t.Errorf("OpportunityID = %q, want %q", outline.OpportunityID, "TEST-001")
	}
	if len(outline.Sections) == 0 {
		t.Error("Outline must contain at least one section")
	}
	if outline.GeneratedAt.IsZero() {
		t.Error("GeneratedAt must be set")
	}
}

// TestOutlineAgent_NilOpportunity verifies the agent returns a failed result and nil outline.
func TestOutlineAgent_NilOpportunity(t *testing.T) {
	ctx := context.Background()
	a := New()

	outline, result, err := a.Run(ctx, nil)

	if err == nil {
		t.Fatal("Run() should return an error when opportunity is nil")
	}
	if result == nil {
		t.Fatal("Run() should still return a Result even on failure")
	}
	if result.Status != agent.StatusFailed {
		t.Errorf("Status = %q, want %q", result.Status, agent.StatusFailed)
	}
	if result.AgentName != agentName {
		t.Errorf("AgentName = %q, want %q", result.AgentName, agentName)
	}
	const wantSummary = "opportunity must not be nil"
	if result.Summary != wantSummary {
		t.Errorf("Summary = %q, want %q", result.Summary, wantSummary)
	}
	if outline != nil {
		t.Error("Run() should return nil outline on failure")
	}
}

// TestBuildSections_Basesections verifies that the five standard federal proposal
// volumes are always present, even for a sparse opportunity.
func TestBuildSections_BaseSections(t *testing.T) {
	opp := baseOpportunity()
	opp.Description = "" // sparse: no description
	opp.SetAsideCode = ""

	sections := buildSections(opp)

	required := []string{
		"executive_summary",
		"technical_approach",
		"management_approach",
		"past_performance",
		"price_cost_volume",
	}
	ids := sectionIDs(sections)
	for _, id := range required {
		if !contains(ids, id) {
			t.Errorf("base section %q missing from sparse opportunity", id)
		}
	}
}

// TestBuildSections_SetAside verifies a small business subcontracting plan is added
// when a set-aside code is present.
func TestBuildSections_SetAside(t *testing.T) {
	opp := baseOpportunity()
	opp.SetAsideCode = "SBA"

	sections := buildSections(opp)

	if !contains(sectionIDs(sections), "small_business_subcontracting") {
		t.Error("expected small_business_subcontracting section for SBA set-aside")
	}
}

// TestBuildSections_NoSetAside verifies the subcontracting plan is omitted when
// there is no set-aside.
func TestBuildSections_NoSetAside(t *testing.T) {
	opp := baseOpportunity()
	opp.SetAsideCode = ""

	sections := buildSections(opp)

	if contains(sectionIDs(sections), "small_business_subcontracting") {
		t.Error("unexpected small_business_subcontracting section with no set-aside")
	}
}

// TestBuildSections_KeywordsAddSections verifies that description keywords trigger
// the correct conditional sections.
func TestBuildSections_KeywordsAddSections(t *testing.T) {
	cases := []struct {
		name        string
		description string
		wantSection string
	}{
		{
			name:        "key personnel",
			description: "Contractor shall provide key personnel as defined in Section H.",
			wantSection: "key_personnel",
		},
		{
			name:        "quality assurance",
			description: "A Quality Assurance Surveillance Plan (QASP) is required.",
			wantSection: "quality_assurance",
		},
		{
			name:        "security clearance",
			description: "Personnel must hold an active Secret clearance.",
			wantSection: "security_plan",
		},
		{
			name:        "transition scenario",
			description: "Offeror must describe its transition plan from the incumbent contractor.",
			wantSection: "transition_plan",
		},
		{
			name:        "data rights",
			description: "All technical data produced under this contract is subject to unlimited data rights.",
			wantSection: "data_rights",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opp := baseOpportunity()
			opp.Description = tc.description

			sections := buildSections(opp)

			if !contains(sectionIDs(sections), tc.wantSection) {
				t.Errorf("expected section %q when description is %q", tc.wantSection, tc.description)
			}
		})
	}
}

// TestBuildSections_SectionRationaleSet verifies every returned section has a non-empty
// rationale so downstream agents and the UI can explain the structure.
func TestBuildSections_SectionRationaleSet(t *testing.T) {
	opp := baseOpportunity()
	opp.SetAsideCode = "8A"
	opp.Description = "key personnel required. quality assurance plan mandatory."

	for _, s := range buildSections(opp) {
		if s.Rationale == "" {
			t.Errorf("section %q has empty Rationale", s.ID)
		}
	}
}

// sectionIDs extracts the ID field from a slice of sections.
func sectionIDs(sections []Section) []string {
	ids := make([]string, len(sections))
	for i, s := range sections {
		ids[i] = s.ID
	}
	return ids
}

// contains reports whether slice contains target.
func contains(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}
