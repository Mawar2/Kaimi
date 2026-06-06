package profile

import (
	"testing"

	"github.com/Mawar2/Kaimi/internal/opportunity"
)

// TestIsEligible verifies the eligibility gate against BlueMeta's capability profile.
func TestIsEligible(t *testing.T) {
	tests := []struct {
		name         string
		setAsideCode string
		wantEligible bool
	}{
		// Full-and-open: always eligible
		{name: "full-and-open empty string", setAsideCode: "", wantEligible: true},

		// Small business set-asides: eligible
		{name: "small business (SBA)", setAsideCode: "SBA", wantEligible: true},
		{name: "partial small business (SBP)", setAsideCode: "SBP", wantEligible: true},

		// 8(a): BlueMeta does not hold this certification
		{name: "8(a) set-aside", setAsideCode: "8A", wantEligible: false},
		{name: "8(a) sole source", setAsideCode: "8AN", wantEligible: false},

		// SDVOSB: BlueMeta does not hold this certification
		{name: "SDVOSB set-aside", setAsideCode: "SDVOSB", wantEligible: false},
		{name: "SDVOSB sole source", setAsideCode: "SDVOSBS", wantEligible: false},

		// WOSB / EDWOSB: BlueMeta does not hold these certifications
		{name: "WOSB set-aside", setAsideCode: "WOSB", wantEligible: false},
		{name: "WOSB sole source", setAsideCode: "WOSBSS", wantEligible: false},
		{name: "EDWOSB set-aside", setAsideCode: "EDWOSB", wantEligible: false},
		{name: "EDWOSB sole source", setAsideCode: "EDWOSBSS", wantEligible: false},

		// HUBZone: BlueMeta does not hold this certification
		{name: "HUBZone set-aside", setAsideCode: "HZC", wantEligible: false},
		{name: "HUBZone sole source", setAsideCode: "HZS", wantEligible: false},

		// SDB is NOT gated here — left for Scorer to weight to avoid starving the pipeline
		{name: "SDB not gated (passes through)", setAsideCode: "SDB", wantEligible: true},

		// Case insensitivity and whitespace handling
		{name: "lowercase 8a is ineligible", setAsideCode: "8a", wantEligible: false},
		{name: "lowercase sba is eligible", setAsideCode: "sba", wantEligible: true},
		{name: "whitespace trimmed before check", setAsideCode: "  SBA  ", wantEligible: true},
		{name: "whitespace only treated as full-and-open", setAsideCode: "   ", wantEligible: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opp := &opportunity.Opportunity{
				ID:           "test-opp",
				SetAsideCode: tt.setAsideCode,
			}
			got := BlueMeta.IsEligible(opp)
			if got != tt.wantEligible {
				t.Errorf("IsEligible(%q) = %v, want %v", tt.setAsideCode, got, tt.wantEligible)
			}
		})
	}
}

// TestIsEligible_FixtureOpportunities verifies eligibility against the three fixture opportunities
// used in samgov cached-mode tests. This documents the expected gate outcome for each.
//
// Fixture set-asides:
//   - a1b2c3d4e5f6: "SBA"  → eligible (small business)
//   - f6e5d4c3b2a1: "8A"   → ineligible (8(a) program)
//   - 9z8y7x6w5v4u: ""     → eligible (full-and-open)
func TestIsEligible_FixtureOpportunities(t *testing.T) {
	tests := []struct {
		name         string
		noticeID     string
		setAsideCode string
		wantEligible bool
	}{
		{
			name:         "SBA opportunity kept",
			noticeID:     "a1b2c3d4e5f6",
			setAsideCode: "SBA",
			wantEligible: true,
		},
		{
			name:         "8(a) opportunity dropped",
			noticeID:     "f6e5d4c3b2a1",
			setAsideCode: "8A",
			wantEligible: false,
		},
		{
			name:         "full-and-open opportunity kept",
			noticeID:     "9z8y7x6w5v4u",
			setAsideCode: "",
			wantEligible: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opp := &opportunity.Opportunity{
				ID:           tt.noticeID,
				SetAsideCode: tt.setAsideCode,
			}
			got := BlueMeta.IsEligible(opp)
			if got != tt.wantEligible {
				t.Errorf("IsEligible for %s (set-aside %q) = %v, want %v",
					tt.noticeID, tt.setAsideCode, got, tt.wantEligible)
			}
		})
	}
}

// TestBlueMeta_NAICSCodes verifies the BlueMeta profile has the required NAICS codes.
func TestBlueMeta_NAICSCodes(t *testing.T) {
	allCodes := BlueMeta.AllNAICSCodes()
	if len(allCodes) == 0 {
		t.Fatal("BlueMeta profile must define at least one NAICS code")
	}

	// Primary codes must always be present
	required := []string{"541512", "541519"}
	codeSet := make(map[string]bool, len(allCodes))
	for _, code := range allCodes {
		if code == "" {
			t.Error("BlueMeta AllNAICSCodes must not contain empty strings")
		}
		codeSet[code] = true
	}

	for _, code := range required {
		if !codeSet[code] {
			t.Errorf("BlueMeta profile is missing required NAICS code %q", code)
		}
	}
}

// TestBlueMeta_NAICSCodesTiered verifies that the NAICSCodes slice has tiered entries.
func TestBlueMeta_NAICSCodesTiered(t *testing.T) {
	if len(BlueMeta.NAICSCodes) == 0 {
		t.Fatal("BlueMeta.NAICSCodes must not be empty")
	}

	hasPrimary := false
	for _, n := range BlueMeta.NAICSCodes {
		if n.Code == "" {
			t.Errorf("NAICSCode entry has empty code (description: %q)", n.Description)
		}
		if n.Tier == TierPrimary {
			hasPrimary = true
		}
	}

	if !hasPrimary {
		t.Error("BlueMeta profile must have at least one primary NAICS code")
	}
}

// TestBlueMeta_SetAside verifies the BlueMeta profile has correct set-aside status.
func TestBlueMeta_SetAside(t *testing.T) {
	// BlueMeta is a small business (eligible for SBA/SBP set-asides)
	if !BlueMeta.SetAside.SmallBusiness {
		t.Error("BlueMeta must have SmallBusiness set to true")
	}

	// BlueMeta does not hold these certifications
	if BlueMeta.SetAside.EightA {
		t.Error("BlueMeta must not have EightA set to true")
	}
	if BlueMeta.SetAside.WOSB {
		t.Error("BlueMeta must not have WOSB set to true")
	}
	if BlueMeta.SetAside.HUBZone {
		t.Error("BlueMeta must not have HUBZone set to true")
	}
}

// TestLoadProfile_JSONYAMLParity verifies that JSON and YAML fixtures produce identical profiles.
func TestLoadProfile_JSONYAMLParity(t *testing.T) {
	jsonProfile, err := LoadProfile("testdata/profile_test.json")
	if err != nil {
		t.Fatalf("LoadProfile(json) failed: %v", err)
	}

	yamlProfile, err := LoadProfile("testdata/profile_test.yaml")
	if err != nil {
		t.Fatalf("LoadProfile(yaml) failed: %v", err)
	}

	// Core identifiers must match
	if jsonProfile.UEI != yamlProfile.UEI {
		t.Errorf("UEI mismatch: json=%q yaml=%q", jsonProfile.UEI, yamlProfile.UEI)
	}
	if jsonProfile.Company != yamlProfile.Company {
		t.Errorf("Company mismatch: json=%q yaml=%q", jsonProfile.Company, yamlProfile.Company)
	}

	// NAICS codes must match in count and content
	if len(jsonProfile.NAICSCodes) != len(yamlProfile.NAICSCodes) {
		t.Fatalf("NAICSCodes length mismatch: json=%d yaml=%d",
			len(jsonProfile.NAICSCodes), len(yamlProfile.NAICSCodes))
	}
	for i, jCode := range jsonProfile.NAICSCodes {
		yCode := yamlProfile.NAICSCodes[i]
		if jCode.Code != yCode.Code {
			t.Errorf("NAICSCode[%d].Code mismatch: json=%q yaml=%q", i, jCode.Code, yCode.Code)
		}
		if jCode.Tier != yCode.Tier {
			t.Errorf("NAICSCode[%d].Tier mismatch: json=%q yaml=%q", i, jCode.Tier, yCode.Tier)
		}
		if jCode.Description != yCode.Description {
			t.Errorf("NAICSCode[%d].Description mismatch: json=%q yaml=%q", i, jCode.Description, yCode.Description)
		}
	}

	// Set-aside status must match
	if jsonProfile.SetAside != yamlProfile.SetAside {
		t.Errorf("SetAside mismatch: json=%+v yaml=%+v", jsonProfile.SetAside, yamlProfile.SetAside)
	}

	// Past performance count must match
	if len(jsonProfile.PastPerformance) != len(yamlProfile.PastPerformance) {
		t.Errorf("PastPerformance length mismatch: json=%d yaml=%d",
			len(jsonProfile.PastPerformance), len(yamlProfile.PastPerformance))
	}
}

// TestLoadProfile_RealProfile verifies the real BlueMeta profile.json at the repo root.
// The profile must have 9 NAICS codes and 5 past-performance entries.
func TestLoadProfile_RealProfile(t *testing.T) {
	// ../../profile.json is the repo root when tests run from internal/profile/
	p, err := LoadProfile("../../profile.json")
	if err != nil {
		t.Fatalf("LoadProfile(profile.json) failed: %v", err)
	}

	const wantNAICS = 9
	if len(p.NAICSCodes) != wantNAICS {
		t.Errorf("Expected %d NAICS codes, got %d", wantNAICS, len(p.NAICSCodes))
	}

	const wantPastPerf = 5
	if len(p.PastPerformance) != wantPastPerf {
		t.Errorf("Expected %d past-performance entries, got %d", wantPastPerf, len(p.PastPerformance))
	}

	// Primary codes must be present
	allCodes := p.AllNAICSCodes()
	required := map[string]bool{"541512": false, "541519": false}
	for _, code := range allCodes {
		if _, ok := required[code]; ok {
			required[code] = true
		}
	}
	for code, found := range required {
		if !found {
			t.Errorf("Required NAICS code %q not found in real profile", code)
		}
	}

	// Verify all codes are non-empty and have valid tiers
	for i, n := range p.NAICSCodes {
		if n.Code == "" {
			t.Errorf("NAICSCodes[%d] has empty code", i)
		}
		if n.Tier != TierPrimary && n.Tier != TierSecondary && n.Tier != TierTertiary {
			t.Errorf("NAICSCodes[%d] has invalid tier %q", i, n.Tier)
		}
	}
}

// TestLoadProfile_UnsupportedExtension verifies that an unsupported file extension returns an error.
func TestLoadProfile_UnsupportedExtension(t *testing.T) {
	_, err := LoadProfile("testdata/profile_test.json.bak")
	if err == nil {
		t.Error("Expected error for unsupported file extension, got nil")
	}
}
