package profile_test

import (
	"testing"

	"github.com/Mawar2/Kaimi/internal/profile"
)

// TestAllNAICSCodes verifies deduplication and ordering across tiers.
func TestAllNAICSCodes(t *testing.T) {
	p := &profile.CapabilityProfile{
		NAICS: profile.NAICSTiers{
			Primary:   "541512",
			Secondary: []string{"541519", "541511"},
			Tertiary:  []string{"518210", "541512"}, // 541512 duplicates primary
		},
	}

	codes := p.AllNAICSCodes()

	// Should have 4 unique codes (541512 deduplicated)
	if len(codes) != 4 {
		t.Errorf("Expected 4 unique NAICS codes, got %d: %v", len(codes), codes)
	}

	// Primary must be first
	if len(codes) > 0 && codes[0] != "541512" {
		t.Errorf("Expected primary code first, got %q", codes[0])
	}
}

// TestAllNAICSCodes_EmptyProfile verifies that an empty profile returns no codes.
func TestAllNAICSCodes_EmptyProfile(t *testing.T) {
	p := &profile.CapabilityProfile{}
	codes := p.AllNAICSCodes()
	if len(codes) != 0 {
		t.Errorf("Expected no codes from empty profile, got %v", codes)
	}
}

// TestLoadProfile_JSON verifies JSON loading produces a valid profile.
func TestLoadProfile_JSON(t *testing.T) {
	p, err := profile.LoadProfile("testdata/profile_test.json")
	if err != nil {
		t.Fatalf("Failed to load JSON profile: %v", err)
	}
	if p.Name != "Test Company" {
		t.Errorf("Expected name %q, got %q", "Test Company", p.Name)
	}
	if !p.SetAside.SmallBusiness {
		t.Error("Expected small_business to be true")
	}
	if len(p.AllNAICSCodes()) != 3 {
		t.Errorf("Expected 3 NAICS codes, got %d", len(p.AllNAICSCodes()))
	}
}

// TestLoadProfile_YAML verifies YAML loading produces the same result as JSON.
func TestLoadProfile_YAML(t *testing.T) {
	p, err := profile.LoadProfile("testdata/profile_test.yaml")
	if err != nil {
		t.Fatalf("Failed to load YAML profile: %v", err)
	}
	if p.Name != "Test Company" {
		t.Errorf("Expected name %q, got %q", "Test Company", p.Name)
	}
	if !p.SetAside.SmallBusiness {
		t.Error("Expected small_business to be true")
	}
	if len(p.AllNAICSCodes()) != 3 {
		t.Errorf("Expected 3 NAICS codes, got %d", len(p.AllNAICSCodes()))
	}
}

// TestLoadProfile_NotFound verifies an error is returned for a missing file.
func TestLoadProfile_NotFound(t *testing.T) {
	_, err := profile.LoadProfile("testdata/nonexistent.json")
	if err == nil {
		t.Error("Expected error for missing profile file")
	}
}

// TestLoadProfile_RealProfile loads profile.json from the project root and verifies
// it contains 9 NAICS codes (1 primary + 5 secondary + 3 tertiary) and 5
// past-performance entries.
func TestLoadProfile_RealProfile(t *testing.T) {
	// Tests run from the package directory; profile.json is two levels up.
	possiblePaths := []string{
		"../../profile.json",
		"profile.json",
	}

	var p *profile.CapabilityProfile
	var err error
	for _, path := range possiblePaths {
		p, err = profile.LoadProfile(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("Failed to load project profile.json: %v", err)
	}

	codes := p.AllNAICSCodes()
	if len(codes) != 9 {
		t.Errorf("Expected 9 NAICS codes (1 primary + 5 secondary + 3 tertiary), got %d: %v", len(codes), codes)
	}

	if len(p.PastPerformance) != 5 {
		t.Errorf("Expected 5 past-performance entries, got %d", len(p.PastPerformance))
	}

	if p.Name == "" {
		t.Error("Expected non-empty profile name")
	}
}
