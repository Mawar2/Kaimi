package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mawar2/Kaimi/internal/profile"
	"github.com/Mawar2/Kaimi/internal/store"
)

// TestIsEligible covers all set-aside code families handled by isEligible.
// 17 sub-cases: full-and-open, small-business, 8(a), SDVOSB, WOSB, HUBZone,
// VOSB, IEE/ISBEE, and one unrecognized code.
func TestIsEligible(t *testing.T) {
	tests := []struct {
		code     string
		eligible bool
	}{
		// Full-and-open: always eligible
		{"", true},
		{"NONE", true},
		// Small Business set-asides — BlueMeta qualifies
		{"SBA", true},
		{"SBP", true},
		// 8(a) Program — BlueMeta does not hold this certification
		{"8A", false},
		{"8(A)", false},
		{"8AN", false},
		// Service-Disabled Veteran-Owned
		{"SDVOSB", false},
		{"SDVOSBC", false},
		// Women-Owned
		{"WOSB", false},
		{"EDWOSB", false},
		// HUBZone
		{"HUBZONE", false},
		{"HUB", false},
		// Veteran-Owned
		{"VOSB", false},
		// Indian Economic Enterprise
		{"IEE", false},
		{"ISBEE", false},
		// Unrecognized — pass through to avoid false negatives
		{"UNKNOWN_XYZ", true},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := isEligible(tt.code)
			if got != tt.eligible {
				t.Errorf("isEligible(%q) = %v, want %v", tt.code, got, tt.eligible)
			}
		})
	}
}

// TestIsEligible_CaseNormalization verifies that set-aside codes are matched
// case-insensitively with whitespace trimmed.
func TestIsEligible_CaseNormalization(t *testing.T) {
	tests := []struct {
		code     string
		eligible bool
	}{
		{"sba", true},     // lowercase eligible
		{"sdvosb", false}, // lowercase ineligible
		{"  SBA  ", true}, // whitespace trimmed
		{"None", true},    // mixed-case NONE (from SAM.gov fixture)
		{"   ", true},     // whitespace-only treated as full-and-open
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := isEligible(tt.code)
			if got != tt.eligible {
				t.Errorf("isEligible(%q) = %v, want %v", tt.code, got, tt.eligible)
			}
		})
	}
}

// TestValidateConfig verifies configuration validation after replacing the
// --naics flag with --profile.
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		shouldError bool
	}{
		{
			name: "valid cached config",
			config: Config{
				Mode:        "cached",
				ProfilePath: "profile.json",
				StoreType:   "json",
				StorePath:   "./queue",
			},
			shouldError: false,
		},
		{
			name: "valid live config with API key",
			config: Config{
				Mode:        "live",
				APIKey:      "test-api-key",
				ProfilePath: "profile.json",
				StoreType:   "json",
				StorePath:   "./queue",
			},
			shouldError: false,
		},
		{
			name: "invalid mode",
			config: Config{
				Mode:        "invalid",
				ProfilePath: "profile.json",
				StoreType:   "json",
			},
			shouldError: true,
		},
		{
			name: "live mode without API key",
			config: Config{
				Mode:        "live",
				ProfilePath: "profile.json",
				StoreType:   "json",
			},
			shouldError: true,
		},
		{
			name: "empty profile path",
			config: Config{
				Mode:      "cached",
				StoreType: "json",
			},
			shouldError: true,
		},
		{
			name: "unsupported store type",
			config: Config{
				Mode:        "cached",
				ProfilePath: "profile.json",
				StoreType:   "firestore",
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(&tt.config)
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestGetEnv verifies environment variable reading with defaults.
func TestGetEnv(t *testing.T) {
	testKey := "TEST_HUNTER_VAR"
	testValue := "test-value"
	if err := os.Setenv(testKey, testValue); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv(testKey); err != nil {
			t.Errorf("Failed to unset environment variable: %v", err)
		}
	}()

	tests := []struct {
		name         string
		key          string
		defaultValue string
		expected     string
	}{
		{"existing variable", testKey, "default", testValue},
		{"non-existent variable", "NONEXISTENT_VAR", "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getEnv(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// writeProfileJSON serialises a CapabilityProfile to a temp file and returns the path.
func writeProfileJSON(t *testing.T, p *profile.CapabilityProfile) string {
	t.Helper()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Failed to marshal profile: %v", err)
	}
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("Failed to write profile: %v", err)
	}
	return path
}

// TestRunWithConfig exercises the full Hunter pipeline in cached mode using a
// temp profile and a temp store directory.
//
// Fixture set-aside codes (test/fixtures/samgov_response.json):
//   - a1b2c3d4e5f6: NAICS 541512, set-aside "SBA"  → eligible
//   - f6e5d4c3b2a1: NAICS 541512, set-aside "8A"   → ineligible (dropped)
//   - 9z8y7x6w5v4u: NAICS 541519, set-aside "None" → eligible
func TestRunWithConfig(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Write a minimal profile covering both fixture NAICS codes.
	prof := &profile.CapabilityProfile{
		Name:     "Test Profile",
		SetAside: profile.SetAsideStatus{SmallBusiness: true},
		NAICS: profile.NAICSTiers{
			Primary:   "541512",
			Secondary: []string{"541519"},
		},
	}
	profilePath := writeProfileJSON(t, prof)

	config := Config{
		Mode:        "cached",
		ProfilePath: profilePath,
		StoreType:   "json",
		StorePath:   tempDir,
	}

	if err := runWithConfig(ctx, &config); err != nil {
		t.Fatalf("runWithConfig failed: %v", err)
	}

	// Open the store to verify saved opportunities.
	opportunityStore, err := store.NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}

	saved, err := opportunityStore.List(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list saved opportunities: %v", err)
	}

	// 3 fetched: SBA (eligible) + 8A (dropped) + None (eligible) = 2 saved.
	if len(saved) != 2 {
		t.Errorf("Expected 2 eligible opportunities saved, got %d", len(saved))
	}

	ids := make(map[string]bool, len(saved))
	for _, opp := range saved {
		ids[opp.ID] = true
	}

	if ids["f6e5d4c3b2a1"] {
		t.Error("8(a) opportunity f6e5d4c3b2a1 should have been dropped by eligibility gate")
	}
	if !ids["a1b2c3d4e5f6"] {
		t.Error("SBA opportunity a1b2c3d4e5f6 should have been saved")
	}
	if !ids["9z8y7x6w5v4u"] {
		t.Error("Full-and-open opportunity 9z8y7x6w5v4u should have been saved")
	}
}

// TestRunWithConfig_NoMatches verifies runWithConfig handles gracefully when no
// opportunities match the profile's NAICS codes.
func TestRunWithConfig_NoMatches(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Profile with a NAICS code absent from the cached fixtures.
	prof := &profile.CapabilityProfile{
		Name:  "No-Match Profile",
		NAICS: profile.NAICSTiers{Primary: "999999"},
	}
	profilePath := writeProfileJSON(t, prof)

	config := Config{
		Mode:        "cached",
		ProfilePath: profilePath,
		StoreType:   "json",
		StorePath:   tempDir,
	}

	if err := runWithConfig(ctx, &config); err != nil {
		t.Fatalf("runWithConfig failed: %v", err)
	}

	opportunityStore, err := store.NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}

	all, err := opportunityStore.List(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list opportunities: %v", err)
	}

	if len(all) != 0 {
		t.Errorf("Expected empty store for non-matching NAICS, got %d opportunities", len(all))
	}
}
