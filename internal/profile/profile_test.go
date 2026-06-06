package profile

import (
	"path/filepath"
	"testing"

	"github.com/Mawar2/Kaimi/internal/opportunity"
)

// TestIsEligible verifies the eligibility switch against all known SAM.gov set-aside families.
func TestIsEligible(t *testing.T) {
	p := &CapabilityProfile{}

	tests := []struct {
		name         string
		setAsideCode string
		wantEligible bool
	}{
		// Full-and-open: always eligible
		{"empty string (full-and-open)", "", true},
		{"NONE", "NONE", true},

		// Small business set-asides: eligible
		{"SBA (small business)", "SBA", true},
		{"SBP (partial small business)", "SBP", true},
		{"SDB (small disadvantaged)", "SDB", true},

		// 8(a): BlueMeta not certified
		{"8A set-aside", "8A", false},
		{"8(A) set-aside", "8(A)", false},
		{"8AN sole source", "8AN", false},

		// SDVOSB: not held
		{"SDVOSB set-aside", "SDVOSB", false},
		{"SDVOSBC (SDVOSB competitive)", "SDVOSBC", false},

		// WOSB / EDWOSB: not held
		{"WOSB set-aside", "WOSB", false},
		{"EDWOSB set-aside", "EDWOSB", false},

		// HUBZone: not held
		{"HUBZONE set-aside", "HUBZONE", false},
		{"HUB set-aside", "HUB", false},

		// VOSB: not held
		{"VOSB set-aside", "VOSB", false},

		// Indian enterprise: not held
		{"IEE set-aside", "IEE", false},
		{"ISBEE set-aside", "ISBEE", false},

		// Unrecognized code: conservative passthrough (eligible)
		{"unrecognized code passes through", "XYZ_UNKNOWN_CODE", true},

		// Legacy SAM.gov codes: backward compatibility
		{"HZC (legacy HUBZone set-aside)", "HZC", false},
		{"HZS (legacy HUBZone sole source)", "HZS", false},
		{"SDVOSBS (legacy SDVOSB sole source)", "SDVOSBS", false},
		{"WOSBSS (legacy WOSB sole source)", "WOSBSS", false},
		{"EDWOSBSS (legacy EDWOSB sole source)", "EDWOSBSS", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opp := &opportunity.Opportunity{
				ID:           "test-opp",
				SetAsideCode: tt.setAsideCode,
			}
			got := p.IsEligible(opp)
			if got != tt.wantEligible {
				t.Errorf("IsEligible(%q) = %v, want %v", tt.setAsideCode, got, tt.wantEligible)
			}
		})
	}
}

// TestIsEligible_CaseNormalization verifies case-insensitive matching and whitespace trimming.
func TestIsEligible_CaseNormalization(t *testing.T) {
	p := &CapabilityProfile{}

	tests := []struct {
		name         string
		setAsideCode string
		wantEligible bool
	}{
		{"lowercase 8a is ineligible", "8a", false},
		{"mixed-case Sdvosb is ineligible", "Sdvosb", false},
		{"lowercase wosb is ineligible", "wosb", false},
		{"lowercase hubzone is ineligible", "hubzone", false},
		{"lowercase sba is eligible", "sba", true},
		{"mixed-case Sba is eligible", "Sba", true},
		{"lowercase none is eligible", "none", true},
		{"whitespace around SBA trimmed", "  SBA  ", true},
		{"whitespace around 8A trimmed (ineligible)", "  8A  ", false},
		{"whitespace-only treated as full-and-open", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opp := &opportunity.Opportunity{
				ID:           "test-opp",
				SetAsideCode: tt.setAsideCode,
			}
			got := p.IsEligible(opp)
			if got != tt.wantEligible {
				t.Errorf("IsEligible(%q) = %v, want %v", tt.setAsideCode, got, tt.wantEligible)
			}
		})
	}
}

// TestAllNAICSCodes verifies that AllNAICSCodes returns a flat slice of code strings
// in order, regardless of tier.
func TestAllNAICSCodes(t *testing.T) {
	p := &CapabilityProfile{
		NAICSCodes: []NAICSCode{
			{Code: "541512", Tier: TierPrimary},
			{Code: "518210", Tier: TierSecondary},
			{Code: "541513", Tier: TierTertiary},
		},
	}

	codes := p.AllNAICSCodes()
	if len(codes) != 3 {
		t.Fatalf("AllNAICSCodes() returned %d codes, want 3", len(codes))
	}
	if codes[0] != "541512" {
		t.Errorf("codes[0] = %q, want %q", codes[0], "541512")
	}
	if codes[1] != "518210" {
		t.Errorf("codes[1] = %q, want %q", codes[1], "518210")
	}
	if codes[2] != "541513" {
		t.Errorf("codes[2] = %q, want %q", codes[2], "541513")
	}
}

// TestAllNAICSCodes_Empty verifies AllNAICSCodes returns an empty slice for empty profile.
func TestAllNAICSCodes_Empty(t *testing.T) {
	p := &CapabilityProfile{}
	codes := p.AllNAICSCodes()
	if len(codes) != 0 {
		t.Errorf("AllNAICSCodes() on empty profile returned %d codes, want 0", len(codes))
	}
}

// TestLoadProfile_JSONYAMLParity verifies that JSON and YAML fixtures with identical
// content produce equivalent CapabilityProfile structs.
func TestLoadProfile_JSONYAMLParity(t *testing.T) {
	jsonPath := filepath.Join("testdata", "profile_test.json")
	yamlPath := filepath.Join("testdata", "profile_test.yaml")

	jsonProfile, err := LoadProfile(jsonPath)
	if err != nil {
		t.Fatalf("LoadProfile(%q) error: %v", jsonPath, err)
	}

	yamlProfile, err := LoadProfile(yamlPath)
	if err != nil {
		t.Fatalf("LoadProfile(%q) error: %v", yamlPath, err)
	}

	if jsonProfile.Company != yamlProfile.Company {
		t.Errorf("Company: JSON=%q, YAML=%q", jsonProfile.Company, yamlProfile.Company)
	}
	if jsonProfile.UEI != yamlProfile.UEI {
		t.Errorf("UEI: JSON=%q, YAML=%q", jsonProfile.UEI, yamlProfile.UEI)
	}
	if len(jsonProfile.NAICSCodes) != len(yamlProfile.NAICSCodes) {
		t.Fatalf("NAICSCodes length: JSON=%d, YAML=%d", len(jsonProfile.NAICSCodes), len(yamlProfile.NAICSCodes))
	}
	for i := range jsonProfile.NAICSCodes {
		jc := jsonProfile.NAICSCodes[i]
		yc := yamlProfile.NAICSCodes[i]
		if jc.Code != yc.Code {
			t.Errorf("NAICSCodes[%d].Code: JSON=%q, YAML=%q", i, jc.Code, yc.Code)
		}
		if jc.Tier != yc.Tier {
			t.Errorf("NAICSCodes[%d].Tier: JSON=%q, YAML=%q", i, jc.Tier, yc.Tier)
		}
	}
	if len(jsonProfile.PastPerformance) != len(yamlProfile.PastPerformance) {
		t.Errorf("PastPerformance length: JSON=%d, YAML=%d",
			len(jsonProfile.PastPerformance), len(yamlProfile.PastPerformance))
	}
	if jsonProfile.SetAside.SmallBusiness != yamlProfile.SetAside.SmallBusiness {
		t.Errorf("SetAside.SmallBusiness: JSON=%v, YAML=%v",
			jsonProfile.SetAside.SmallBusiness, yamlProfile.SetAside.SmallBusiness)
	}
}

// TestLoadProfile_RealProfile verifies the production BlueMeta profile loads correctly
// with 9 NAICS codes and 5 past-performance entries.
func TestLoadProfile_RealProfile(t *testing.T) {
	profilePath := filepath.Join("..", "..", "config", "profile.json")

	p, err := LoadProfile(profilePath)
	if err != nil {
		t.Fatalf("LoadProfile(%q) error: %v", profilePath, err)
	}

	if len(p.NAICSCodes) != 9 {
		t.Errorf("Expected 9 NAICS codes in production profile, got %d", len(p.NAICSCodes))
	}
	if len(p.PastPerformance) != 5 {
		t.Errorf("Expected 5 past-performance entries in production profile, got %d", len(p.PastPerformance))
	}

	// AllNAICSCodes must return same count as NAICSCodes
	codes := p.AllNAICSCodes()
	if len(codes) != len(p.NAICSCodes) {
		t.Errorf("AllNAICSCodes() returned %d codes, want %d", len(codes), len(p.NAICSCodes))
	}
	for _, code := range codes {
		if code == "" {
			t.Error("AllNAICSCodes() returned an empty-string code")
		}
	}
}

// TestLoadProfile_UnsupportedExtension verifies LoadProfile rejects unknown file extensions.
func TestLoadProfile_UnsupportedExtension(t *testing.T) {
	_, err := LoadProfile("profile.toml")
	if err == nil {
		t.Error("Expected error for unsupported extension, got nil")
	}
}
