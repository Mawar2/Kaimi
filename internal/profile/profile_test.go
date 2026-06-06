package profile_test

import (
	"testing"

	"github.com/Mawar2/Kaimi/internal/profile"
)

func TestAllNAICSCodes(t *testing.T) {
	p := &profile.CapabilityProfile{
		NAICS: profile.NAICSTiers{
			Primary:   "541512",
			Secondary: []string{"541519", "541511"},
			Tertiary:  []string{"518210", "541512"}, // 541512 duplicates primary
		},
	}

	codes := p.AllNAICSCodes()

	if len(codes) != 4 {
		t.Errorf("expected 4 unique NAICS codes, got %d: %v", len(codes), codes)
	}
	if codes[0] != "541512" {
		t.Errorf("expected primary code first, got %q", codes[0])
	}
}

func TestAllNAICSCodes_EmptyProfile(t *testing.T) {
	p := &profile.CapabilityProfile{}
	codes := p.AllNAICSCodes()
	if len(codes) != 0 {
		t.Errorf("expected no codes from empty profile, got %v", codes)
	}
}

func TestLoadProfile_JSON(t *testing.T) {
	p, err := profile.LoadProfile("testdata/profile_test.json")
	if err != nil {
		t.Fatalf("failed to load JSON profile: %v", err)
	}
	if p.Name != "Test Company" {
		t.Errorf("expected name %q, got %q", "Test Company", p.Name)
	}
	if !p.SetAside.SmallBusiness {
		t.Error("expected small_business to be true")
	}
	if len(p.AllNAICSCodes()) != 3 {
		t.Errorf("expected 3 NAICS codes, got %d: %v", len(p.AllNAICSCodes()), p.AllNAICSCodes())
	}
}

func TestLoadProfile_YAML(t *testing.T) {
	p, err := profile.LoadProfile("testdata/profile_test.yaml")
	if err != nil {
		t.Fatalf("failed to load YAML profile: %v", err)
	}
	if p.Name != "Test Company" {
		t.Errorf("expected name %q, got %q", "Test Company", p.Name)
	}
	if !p.SetAside.SmallBusiness {
		t.Error("expected small_business to be true")
	}
	if len(p.AllNAICSCodes()) != 3 {
		t.Errorf("expected 3 NAICS codes, got %d: %v", len(p.AllNAICSCodes()), p.AllNAICSCodes())
	}
}

func TestLoadProfile_NotFound(t *testing.T) {
	_, err := profile.LoadProfile("testdata/nonexistent.json")
	if err == nil {
		t.Error("expected error for missing profile file")
	}
}

// TestLoadProfile_RealProfile loads profile.json from the project root and verifies
// it has 9 NAICS codes (1 primary + 5 secondary + 3 tertiary) and 5 past-performance entries.
func TestLoadProfile_RealProfile(t *testing.T) {
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
		t.Fatalf("failed to load project profile.json: %v", err)
	}

	codes := p.AllNAICSCodes()
	if len(codes) != 9 {
		t.Errorf("expected 9 NAICS codes (1 primary + 5 secondary + 3 tertiary), got %d: %v", len(codes), codes)
	}
	if len(p.PastPerformance) != 5 {
		t.Errorf("expected 5 past-performance entries, got %d", len(p.PastPerformance))
	}
	if p.Name == "" {
		t.Error("expected non-empty profile name")
	}
}
