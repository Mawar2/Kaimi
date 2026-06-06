package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/store"
)

// minimalProfileJSON is a small valid profile used by tests that need a real file
// on disk but do not care about its NAICS contents (the cached SAM.gov client
// returns fixture data regardless of which codes are queried).
const minimalProfileJSON = `{
  "company": "Test Co",
  "naics_codes": [
    {"code": "541512", "description": "Computer Systems Design", "tier": "primary"},
    {"code": "541519", "description": "Other Computer Related",  "tier": "secondary"}
  ],
  "set_aside": {"small_business": true},
  "past_performance": []
}`

// writeProfile writes content to a temp file and returns its path.
func writeProfile(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "profile.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeProfile: %v", err)
	}
	return path
}

// ---- isEligible: 17 set-aside code families + case normalization ----

// TestIsEligible verifies the eligibility gate for all known SAM.gov set-aside
// code families and the conservative unrecognized-code passthrough.
func TestIsEligible(t *testing.T) {
	tests := []struct {
		name         string
		setAsideCode string
		wantEligible bool
	}{
		// Full-and-open: always eligible
		{name: "empty string (full-and-open)", setAsideCode: "", wantEligible: true},
		{name: "NONE (explicit full-and-open)", setAsideCode: "NONE", wantEligible: true},

		// Small business set-asides: eligible
		{name: "SBA (total small business)", setAsideCode: "SBA", wantEligible: true},
		{name: "SBP (partial small business)", setAsideCode: "SBP", wantEligible: true},

		// 8(a): not held
		{name: "8A set-aside", setAsideCode: "8A", wantEligible: false},
		{name: "8(A) variant", setAsideCode: "8(A)", wantEligible: false},
		{name: "8AN sole source", setAsideCode: "8AN", wantEligible: false},

		// SDVOSB: not held
		{name: "SDVOSB set-aside", setAsideCode: "SDVOSB", wantEligible: false},
		{name: "SDVOSBC competitive", setAsideCode: "SDVOSBC", wantEligible: false},

		// WOSB / EDWOSB: not held
		{name: "WOSB set-aside", setAsideCode: "WOSB", wantEligible: false},
		{name: "EDWOSB set-aside", setAsideCode: "EDWOSB", wantEligible: false},

		// HUBZone: not held
		{name: "HUBZONE set-aside", setAsideCode: "HUBZONE", wantEligible: false},
		{name: "HUB set-aside", setAsideCode: "HUB", wantEligible: false},

		// VOSB: not held
		{name: "VOSB set-aside", setAsideCode: "VOSB", wantEligible: false},

		// Indian enterprise: not held
		{name: "IEE set-aside", setAsideCode: "IEE", wantEligible: false},
		{name: "ISBEE set-aside", setAsideCode: "ISBEE", wantEligible: false},

		// Unrecognized: conservative passthrough to avoid false negatives
		{name: "unrecognized code passes through", setAsideCode: "UNKNOWN_XYZ", wantEligible: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEligible(tt.setAsideCode)
			if got != tt.wantEligible {
				t.Errorf("isEligible(%q) = %v, want %v", tt.setAsideCode, got, tt.wantEligible)
			}
		})
	}
}

// TestIsEligible_CaseNormalization verifies that set-aside codes are matched
// case-insensitively and leading/trailing whitespace is ignored.
func TestIsEligible_CaseNormalization(t *testing.T) {
	tests := []struct {
		name         string
		setAsideCode string
		wantEligible bool
	}{
		{name: "lowercase 8a is ineligible", setAsideCode: "8a", wantEligible: false},
		{name: "lowercase sba is eligible", setAsideCode: "sba", wantEligible: true},
		{name: "mixed-case HubZone is ineligible", setAsideCode: "HubZone", wantEligible: false},
		{name: "whitespace trimmed — SBA is eligible", setAsideCode: "  SBA  ", wantEligible: true},
		{name: "whitespace only treated as full-and-open", setAsideCode: "   ", wantEligible: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEligible(tt.setAsideCode)
			if got != tt.wantEligible {
				t.Errorf("isEligible(%q) = %v, want %v", tt.setAsideCode, got, tt.wantEligible)
			}
		})
	}
}

// ---- filterEligible ----

// TestFilterEligible verifies that filterEligible partitions opportunities
// correctly using the standalone isEligible gate.
func TestFilterEligible(t *testing.T) {
	now := time.Now().UTC()

	makeOpp := func(id, setAside string) *opportunity.Opportunity {
		return &opportunity.Opportunity{
			ID:           id,
			SetAsideCode: setAside,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
	}

	tests := []struct {
		name          string
		opportunities []*opportunity.Opportunity
		wantEligible  int
		wantDropped   int
	}{
		{
			name: "eligible kept, ineligible dropped",
			opportunities: []*opportunity.Opportunity{
				makeOpp("sba-opp", "SBA"),     // eligible
				makeOpp("8a-opp", "8A"),       // ineligible
				makeOpp("open-opp", ""),       // eligible (full-and-open)
				makeOpp("wosb-opp", "WOSB"),   // ineligible
				makeOpp("vosb-opp", "VOSB"),   // ineligible
				makeOpp("isbee-opp", "ISBEE"), // ineligible
			},
			wantEligible: 2,
			wantDropped:  4,
		},
		{
			name: "all eligible",
			opportunities: []*opportunity.Opportunity{
				makeOpp("open1", ""),
				makeOpp("sba1", "SBA"),
				makeOpp("sbp1", "SBP"),
			},
			wantEligible: 3,
			wantDropped:  0,
		},
		{
			name: "all ineligible",
			opportunities: []*opportunity.Opportunity{
				makeOpp("8a1", "8A"),
				makeOpp("sdvosb1", "SDVOSB"),
				makeOpp("hub1", "HUBZONE"),
			},
			wantEligible: 0,
			wantDropped:  3,
		},
		{
			name:          "empty input",
			opportunities: []*opportunity.Opportunity{},
			wantEligible:  0,
			wantDropped:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eligible, dropped := filterEligible(tt.opportunities)
			if len(eligible) != tt.wantEligible {
				t.Errorf("eligible count = %d, want %d", len(eligible), tt.wantEligible)
			}
			if dropped != tt.wantDropped {
				t.Errorf("dropped count = %d, want %d", dropped, tt.wantDropped)
			}
		})
	}
}

// ---- validateConfig ----

// TestValidateConfig verifies configuration validation.
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
				ProfilePath: "./profile.json",
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
				ProfilePath: "./profile.json",
				StoreType:   "json",
				StorePath:   "./queue",
			},
			shouldError: false,
		},
		{
			name: "invalid mode",
			config: Config{
				Mode:        "invalid",
				ProfilePath: "./profile.json",
				StoreType:   "json",
			},
			shouldError: true,
		},
		{
			name: "live mode without API key",
			config: Config{
				Mode:        "live",
				ProfilePath: "./profile.json",
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
				ProfilePath: "./profile.json",
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

// ---- getEnv ----

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
		{
			name:         "existing variable",
			key:          testKey,
			defaultValue: "default",
			expected:     testValue,
		},
		{
			name:         "non-existent variable",
			key:          "NONEXISTENT_VAR",
			defaultValue: "default",
			expected:     "default",
		},
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

// ---- end-to-end: TestRunWithConfig ----

// TestRunWithConfig verifies the full Hunter pipeline in cached mode.
// The SAM.gov cached fixture contains 3 opportunities:
//   - a1b2c3d4e5f6 (SBA)  → eligible
//   - f6e5d4c3b2a1 (8A)   → ineligible, dropped
//   - 9z8y7x6w5v4u ("")   → eligible (full-and-open)
//
// Expected: 3 fetched, 1 dropped, 2 saved.
func TestRunWithConfig(t *testing.T) {
	tempDir := t.TempDir()
	profilePath := writeProfile(t, tempDir, minimalProfileJSON)
	storePath := filepath.Join(tempDir, "queue")

	config := Config{
		Mode:        "cached",
		ProfilePath: profilePath,
		StoreType:   "json",
		StorePath:   storePath,
	}

	if err := runWithConfig(&config); err != nil {
		t.Fatalf("runWithConfig() failed: %v", err)
	}

	// Open the store and verify exactly 2 opportunities were saved.
	ctx := context.Background()
	opportunityStore, err := store.NewJSONStore(storePath)
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}

	saved, err := opportunityStore.List(ctx, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(saved) != 2 {
		t.Errorf("expected 2 saved opportunities (8a filtered), got %d", len(saved))
	}

	// Confirm no 8(a) opportunity slipped through.
	for _, opp := range saved {
		if opp.SetAsideCode == "8A" {
			t.Errorf("ineligible 8(a) opportunity %s was saved", opp.ID)
		}
	}
}

// TestRunWithConfig_MissingProfile verifies that runWithConfig returns an error
// when the profile file does not exist.
func TestRunWithConfig_MissingProfile(t *testing.T) {
	tempDir := t.TempDir()

	config := Config{
		Mode:        "cached",
		ProfilePath: filepath.Join(tempDir, "nonexistent.json"),
		StoreType:   "json",
		StorePath:   filepath.Join(tempDir, "queue"),
	}

	if err := runWithConfig(&config); err == nil {
		t.Error("expected error for missing profile, got nil")
	}
}

// ---- legacy integration coverage ----

// TestHunterIntegration is an end-to-end integration test using the cached
// SAM.gov fixture and a temp store.
func TestHunterIntegration(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	profilePath := writeProfile(t, tempDir, minimalProfileJSON)

	config := Config{
		Mode:        "cached",
		ProfilePath: profilePath,
		StoreType:   "json",
		StorePath:   filepath.Join(tempDir, "queue"),
	}

	if err := runWithConfig(&config); err != nil {
		t.Fatalf("runWithConfig() failed: %v", err)
	}

	opportunityStore, err := store.NewJSONStore(filepath.Join(tempDir, "queue"))
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}

	saved, err := opportunityStore.List(ctx, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(saved) == 0 {
		t.Fatal("expected at least one saved opportunity")
	}

	t.Logf("Integration test complete: %d opportunities saved", len(saved))
}
