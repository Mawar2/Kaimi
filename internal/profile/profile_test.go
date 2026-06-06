package profile

import (
	"path/filepath"
	"runtime"
	"testing"
)

// thisDir returns the directory containing this test file, used to build
// deterministic paths to testdata and the repo-root profile.json.
func thisDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

// TestLoadProfile_JSON verifies loading a CapabilityProfile from a JSON fixture.
func TestLoadProfile_JSON(t *testing.T) {
	p, err := LoadProfile(filepath.Join(thisDir(), "testdata", "profile_test.json"))
	if err != nil {
		t.Fatalf("LoadProfile(.json) failed: %v", err)
	}
	if p.Company == "" {
		t.Error("expected non-empty company name")
	}
	if len(p.NAICSCodes) == 0 {
		t.Error("expected at least one NAICS code")
	}
	if len(p.PastPerformance) == 0 {
		t.Error("expected at least one past performance entry")
	}
}

// TestLoadProfile_YAML verifies loading a CapabilityProfile from a YAML fixture.
func TestLoadProfile_YAML(t *testing.T) {
	p, err := LoadProfile(filepath.Join(thisDir(), "testdata", "profile_test.yaml"))
	if err != nil {
		t.Fatalf("LoadProfile(.yaml) failed: %v", err)
	}
	if p.Company == "" {
		t.Error("expected non-empty company name")
	}
	if len(p.NAICSCodes) == 0 {
		t.Error("expected at least one NAICS code")
	}
}

// TestLoadProfile_JSONYAMLParity verifies that the JSON and YAML fixtures produce
// equivalent profiles — same company, NAICS count, and past-performance count.
func TestLoadProfile_JSONYAMLParity(t *testing.T) {
	jsonProfile, err := LoadProfile(filepath.Join(thisDir(), "testdata", "profile_test.json"))
	if err != nil {
		t.Fatalf("JSON load failed: %v", err)
	}
	yamlProfile, err := LoadProfile(filepath.Join(thisDir(), "testdata", "profile_test.yaml"))
	if err != nil {
		t.Fatalf("YAML load failed: %v", err)
	}

	if jsonProfile.Company != yamlProfile.Company {
		t.Errorf("company mismatch: json=%q yaml=%q", jsonProfile.Company, yamlProfile.Company)
	}
	if len(jsonProfile.NAICSCodes) != len(yamlProfile.NAICSCodes) {
		t.Errorf("NAICS count mismatch: json=%d yaml=%d",
			len(jsonProfile.NAICSCodes), len(yamlProfile.NAICSCodes))
	}
	if len(jsonProfile.PastPerformance) != len(yamlProfile.PastPerformance) {
		t.Errorf("past performance count mismatch: json=%d yaml=%d",
			len(jsonProfile.PastPerformance), len(yamlProfile.PastPerformance))
	}
}

// TestLoadProfile_UnsupportedExtension verifies that unsupported file extensions
// return an error rather than silently producing an empty profile.
func TestLoadProfile_UnsupportedExtension(t *testing.T) {
	_, err := LoadProfile("profile.toml")
	if err == nil {
		t.Error("expected error for unsupported extension, got nil")
	}
}

// TestLoadProfile_NotFound verifies that a missing file returns an error.
func TestLoadProfile_NotFound(t *testing.T) {
	_, err := LoadProfile("/nonexistent/path/profile.json")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

// TestAllNAICSCodes verifies that AllNAICSCodes returns all codes as a flat,
// non-empty string list with the same length as the profile's NAICSCodes slice.
func TestAllNAICSCodes(t *testing.T) {
	p, err := LoadProfile(filepath.Join(thisDir(), "testdata", "profile_test.json"))
	if err != nil {
		t.Fatalf("LoadProfile failed: %v", err)
	}

	codes := p.AllNAICSCodes()
	if len(codes) != len(p.NAICSCodes) {
		t.Errorf("AllNAICSCodes() returned %d codes, want %d", len(codes), len(p.NAICSCodes))
	}
	for i, code := range codes {
		if code == "" {
			t.Errorf("AllNAICSCodes()[%d] is empty", i)
		}
	}
}

// TestLoadProfile_TierFields verifies that tier values round-trip correctly
// from JSON through the NAICSTier type.
func TestLoadProfile_TierFields(t *testing.T) {
	p, err := LoadProfile(filepath.Join(thisDir(), "testdata", "profile_test.json"))
	if err != nil {
		t.Fatalf("LoadProfile failed: %v", err)
	}

	for _, nc := range p.NAICSCodes {
		switch nc.Tier {
		case TierPrimary, TierSecondary, TierTertiary:
			// valid
		default:
			t.Errorf("NAICS code %q has unexpected tier %q", nc.Code, nc.Tier)
		}
	}
}

// TestLoadProfile_RealProfile validates that profile.json (the real BlueMeta profile
// at the repository root) has exactly 9 NAICS codes and 5 past-performance entries.
func TestLoadProfile_RealProfile(t *testing.T) {
	profilePath := filepath.Join(thisDir(), "..", "..", "profile.json")

	p, err := LoadProfile(profilePath)
	if err != nil {
		t.Fatalf("LoadProfile(profile.json) failed: %v", err)
	}

	if len(p.NAICSCodes) != 9 {
		t.Errorf("profile.json: expected 9 NAICS codes, got %d", len(p.NAICSCodes))
	}
	if len(p.PastPerformance) != 5 {
		t.Errorf("profile.json: expected 5 past performance entries, got %d", len(p.PastPerformance))
	}
	if p.Company == "" {
		t.Error("profile.json: company must not be empty")
	}
}
