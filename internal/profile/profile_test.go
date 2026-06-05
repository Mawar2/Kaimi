package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProfile_Success(t *testing.T) {
	// Create a temporary JSON file for testing
	tempDir, err := os.MkdirTemp("", "profile-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	tempFile := filepath.Join(tempDir, "profile.json")
	content := `{
		"uei": "LML1ABC2DEF3",
		"cage": "9C5B2",
		"naics": {
			"primary": ["541512"],
			"secondary": ["541511", "541519"],
			"tertiary": ["541330"]
		},
		"set_aside": {
			"small_business": true,
			"sdb": true,
			"minority_owned": true
		},
		"clearance_status": "Secret",
		"competency_tags": ["identity verification", "mDL"],
		"past_performance": [
			{
				"id": "login-gov",
				"client": "login.gov",
				"scope": "mDL support",
				"value": 1750000.0,
				"what_it_proves": ["mDL", "identity verification"]
			}
		]
	}`

	if err := os.WriteFile(tempFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	prof, err := LoadProfile(tempFile)
	if err != nil {
		t.Fatalf("LoadProfile failed: %v", err)
	}

	if prof.UEI != "LML1ABC2DEF3" {
		t.Errorf("expected UEI 'LML1ABC2DEF3', got %q", prof.UEI)
	}
	if prof.CAGE != "9C5B2" {
		t.Errorf("expected CAGE '9C5B2', got %q", prof.CAGE)
	}
	if len(prof.NAICS.Primary) != 1 || prof.NAICS.Primary[0] != "541512" {
		t.Errorf("unexpected primary NAICS: %v", prof.NAICS.Primary)
	}
	if !prof.SetAside.SmallBusiness || !prof.SetAside.SDB || !prof.SetAside.MinorityOwned {
		t.Errorf("unexpected set aside status: %+v", prof.SetAside)
	}
	if prof.ClearanceStatus != "Secret" {
		t.Errorf("expected clearance 'Secret', got %q", prof.ClearanceStatus)
	}
	if len(prof.CompetencyTags) != 2 || prof.CompetencyTags[0] != "identity verification" {
		t.Errorf("unexpected competency tags: %v", prof.CompetencyTags)
	}
	if len(prof.PastPerformance) != 1 || prof.PastPerformance[0].Client != "login.gov" {
		t.Errorf("unexpected past performance: %+v", prof.PastPerformance)
	}
	if prof.PastPerformance[0].Value != 1750000.0 {
		t.Errorf("expected value 1750000.0, got %f", prof.PastPerformance[0].Value)
	}
}

func TestLoadProfile_YAML_Success(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "profile-test-yaml")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	tempFile := filepath.Join(tempDir, "profile.yaml")
	content := `
uei: LML1ABC2DEF3
cage: 9C5B2
naics:
  primary:
    - "541512"
  secondary:
    - "541511"
    - "541519"
  tertiary:
    - "541330"
set_aside:
  small_business: true
  sdb: true
  minority_owned: true
clearance_status: Secret
competency_tags:
  - "identity verification"
  - "mDL"
past_performance:
  - id: login-gov
    client: login.gov
    scope: mDL support
    value: 1750000.0
    what_it_proves:
      - mDL
      - "identity verification"
`

	if err := os.WriteFile(tempFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	prof, err := LoadProfile(tempFile)
	if err != nil {
		t.Fatalf("LoadProfile failed: %v", err)
	}

	if prof.UEI != "LML1ABC2DEF3" {
		t.Errorf("expected UEI 'LML1ABC2DEF3', got %q", prof.UEI)
	}
	if prof.CAGE != "9C5B2" {
		t.Errorf("expected CAGE '9C5B2', got %q", prof.CAGE)
	}
	if len(prof.NAICS.Primary) != 1 || prof.NAICS.Primary[0] != "541512" {
		t.Errorf("unexpected primary NAICS: %v", prof.NAICS.Primary)
	}
	if !prof.SetAside.SmallBusiness || !prof.SetAside.SDB || !prof.SetAside.MinorityOwned {
		t.Errorf("unexpected set aside status: %+v", prof.SetAside)
	}
	if prof.ClearanceStatus != "Secret" {
		t.Errorf("expected clearance 'Secret', got %q", prof.ClearanceStatus)
	}
}

func TestLoadProfile_InvalidYAML(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "profile-test-invalid-yaml")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	tempFile := filepath.Join(tempDir, "invalid_profile.yaml")
	if err := os.WriteFile(tempFile, []byte(`invalid: [yaml`), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	_, err = LoadProfile(tempFile)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestLoadProfile_FileNotFound(t *testing.T) {
	_, err := LoadProfile("nonexistent_file.json")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestLoadProfile_InvalidJSON(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "profile-test-invalid")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	tempFile := filepath.Join(tempDir, "invalid_profile.json")
	if err := os.WriteFile(tempFile, []byte(`{invalid json`), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	_, err = LoadProfile(tempFile)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestLoadProfile_RealProfile(t *testing.T) {
	// The real profile is in the root directory, which is two levels up from this test file
	realPath := filepath.Join("..", "..", "profile.json")
	prof, err := LoadProfile(realPath)
	if err != nil {
		t.Fatalf("failed to load real profile from %s: %v", realPath, err)
	}

	if prof.UEI == "" {
		t.Error("expected non-empty UEI in real profile")
	}
	if prof.CAGE == "" {
		t.Error("expected non-empty CAGE in real profile")
	}

	// Verify NAICS count matches 9 (1 primary, 5 secondary, 3 tertiary)
	totalNAICS := len(prof.NAICS.Primary) + len(prof.NAICS.Secondary) + len(prof.NAICS.Tertiary)
	if totalNAICS != 9 {
		t.Errorf("expected 9 NAICS codes in real profile, got %d", totalNAICS)
	}

	// Verify past performance has the 5 required entries
	if len(prof.PastPerformance) != 5 {
		t.Errorf("expected 5 past performance entries, got %d", len(prof.PastPerformance))
	}

	expectedClients := map[string]bool{
		"U.S. Census Bureau":                true,
		"Harvard Business School (HBS)":     true,
		"SPRUCE (GSA)":                      true,
		"Defense Intelligence Agency (DIA)": true,
		"login.gov":                         true,
	}

	for _, pp := range prof.PastPerformance {
		if !expectedClients[pp.Client] {
			t.Errorf("unexpected client in past performance: %s", pp.Client)
		}
	}
}
