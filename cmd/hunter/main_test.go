package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/samgov"
	"github.com/Mawar2/Kaimi/internal/store"
)

// TestIsEligible verifies the standalone isEligible function against all known set-aside families.
func TestIsEligible(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		// Full-and-open: always eligible
		{name: "full-and-open empty", code: "", want: true},
		{name: "full-and-open NONE", code: "NONE", want: true},

		// Small business set-asides: eligible
		{name: "small business SBA", code: "SBA", want: true},
		{name: "partial small business SBP", code: "SBP", want: true},

		// 8(a): not held
		{name: "8(a) 8A", code: "8A", want: false},
		{name: "8(a) parenthetical 8(A)", code: "8(A)", want: false},
		{name: "8(a) sole source 8AN", code: "8AN", want: false},

		// SDVOSB: not held
		{name: "SDVOSB competitive", code: "SDVOSB", want: false},
		{name: "SDVOSB competitive SDVOSBC", code: "SDVOSBC", want: false},

		// WOSB / EDWOSB: not held
		{name: "WOSB", code: "WOSB", want: false},
		{name: "EDWOSB", code: "EDWOSB", want: false},

		// HUBZone: not held
		{name: "HUBZone HUBZONE", code: "HUBZONE", want: false},
		{name: "HUBZone HUB", code: "HUB", want: false},

		// VOSB: not held
		{name: "VOSB", code: "VOSB", want: false},

		// Indian enterprise: not held
		{name: "Indian enterprise IEE", code: "IEE", want: false},
		{name: "Indian enterprise ISBEE", code: "ISBEE", want: false},

		// Unrecognized codes pass through to avoid false negatives
		{name: "unrecognized passthrough", code: "XYZ123", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEligible(tt.code)
			if got != tt.want {
				t.Errorf("isEligible(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

// TestIsEligible_CaseNormalization verifies that isEligible normalizes case and whitespace.
func TestIsEligible_CaseNormalization(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{name: "lowercase 8a is ineligible", code: "8a", want: false},
		{name: "lowercase sdvosb is ineligible", code: "sdvosb", want: false},
		{name: "mixed-case Wosb is ineligible", code: "Wosb", want: false},
		{name: "lowercase hubzone is ineligible", code: "hubzone", want: false},
		{name: "lowercase sba is eligible", code: "sba", want: true},
		{name: "leading whitespace trimmed", code: "  SBA", want: true},
		{name: "trailing whitespace trimmed", code: "SBA  ", want: true},
		{name: "whitespace only treated as full-and-open", code: "   ", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEligible(tt.code)
			if got != tt.want {
				t.Errorf("isEligible(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

// TestRunWithConfig verifies the full Hunter pipeline in cached mode:
// 3 fixture opportunities fetched, 1 ineligible (8A) filtered, 2 saved.
func TestRunWithConfig(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &Config{
		Mode:       "cached",
		NAICSCodes: []string{"541512", "541519"},
		StoreType:  "json",
		StorePath:  tempDir,
	}

	if err := runWithConfig(cfg); err != nil {
		t.Fatalf("runWithConfig failed: %v", err)
	}

	// Verify exactly 2 opportunities were saved (8A one filtered)
	ctx := context.Background()
	s, err := store.NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}

	saved, err := s.List(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list saved opportunities: %v", err)
	}

	const wantSaved = 2
	if len(saved) != wantSaved {
		t.Errorf("Expected %d saved opportunities, got %d", wantSaved, len(saved))
	}

	// Verify the 8(a) opportunity was not saved
	for _, opp := range saved {
		if opp.SetAsideCode == "8A" {
			t.Errorf("Ineligible 8(a) opportunity %s must not appear in saved store", opp.ID)
		}
	}
}

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
				Mode:       "cached",
				NAICSCodes: []string{"541512"},
				StoreType:  "json",
				StorePath:  "./queue",
			},
			shouldError: false,
		},
		{
			name: "valid live config with API key",
			config: Config{
				Mode:       "live",
				APIKey:     "test-api-key",
				NAICSCodes: []string{"541512"},
				StoreType:  "json",
				StorePath:  "./queue",
			},
			shouldError: false,
		},
		{
			name: "invalid mode",
			config: Config{
				Mode:       "invalid",
				NAICSCodes: []string{"541512"},
				StoreType:  "json",
			},
			shouldError: true,
		},
		{
			name: "live mode without API key",
			config: Config{
				Mode:       "live",
				NAICSCodes: []string{"541512"},
				StoreType:  "json",
			},
			shouldError: true,
		},
		{
			name: "no NAICS codes",
			config: Config{
				Mode:       "cached",
				NAICSCodes: []string{},
				StoreType:  "json",
			},
			shouldError: true,
		},
		{
			name: "unsupported store type",
			config: Config{
				Mode:       "cached",
				NAICSCodes: []string{"541512"},
				StoreType:  "firestore",
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

// TestHunterIntegration is an end-to-end integration test for the Hunter agent.
func TestHunterIntegration(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "queue")

	samClient, err := samgov.NewClient(samgov.Config{
		UseCached: true,
	})
	if err != nil {
		t.Fatalf("Failed to create SAM.gov client: %v", err)
	}

	opportunityStore, err := store.NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSON store: %v", err)
	}

	naicsCodes := []string{"541512", "541519"}
	opportunities, err := samClient.FetchByNAICS(ctx, naicsCodes)
	if err != nil {
		t.Fatalf("Failed to fetch opportunities: %v", err)
	}

	if len(opportunities) == 0 {
		t.Fatal("Expected to fetch at least one opportunity")
	}

	t.Logf("Fetched %d opportunities", len(opportunities))

	savedCount := 0
	for _, opp := range opportunities {
		if err := opportunityStore.Save(ctx, opp); err != nil {
			t.Errorf("Failed to save opportunity %s: %v", opp.ID, err)
			continue
		}
		savedCount++
	}

	if savedCount != len(opportunities) {
		t.Errorf("Expected to save %d opportunities, saved %d", len(opportunities), savedCount)
	}

	for _, opp := range opportunities {
		retrieved, err := opportunityStore.Get(ctx, opp.ID)
		if err != nil {
			t.Errorf("Failed to retrieve opportunity %s: %v", opp.ID, err)
			continue
		}

		if retrieved.ID != opp.ID {
			t.Errorf("ID mismatch: expected %q, got %q", opp.ID, retrieved.ID)
		}
		if retrieved.Title != opp.Title {
			t.Errorf("Title mismatch for %s: expected %q, got %q", opp.ID, opp.Title, retrieved.Title)
		}
		if retrieved.Agency != opp.Agency {
			t.Errorf("Agency mismatch for %s: expected %q, got %q", opp.ID, opp.Agency, retrieved.Agency)
		}
	}

	entries, err := os.ReadDir(storePath)
	if err != nil {
		t.Fatalf("Failed to read store directory: %v", err)
	}

	jsonFileCount := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			jsonFileCount++
		}
	}

	if jsonFileCount != len(opportunities) {
		t.Errorf("Expected %d JSON files, found %d", len(opportunities), jsonFileCount)
	}

	t.Logf("Integration test complete: %d opportunities saved and verified", savedCount)
}

// TestHunterIntegration_EmptyNAICS verifies Hunter behavior with NAICS codes that return no results.
func TestHunterIntegration_EmptyNAICS(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()

	samClient, err := samgov.NewClient(samgov.Config{
		UseCached: true,
	})
	if err != nil {
		t.Fatalf("Failed to create SAM.gov client: %v", err)
	}

	opportunityStore, err := store.NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSON store: %v", err)
	}

	naicsCodes := []string{"999999"}
	opportunities, err := samClient.FetchByNAICS(ctx, naicsCodes)
	if err != nil {
		t.Fatalf("Failed to fetch opportunities: %v", err)
	}

	if len(opportunities) != 0 {
		t.Errorf("Expected 0 opportunities for non-matching NAICS, got %d", len(opportunities))
	}

	allOpportunities, err := opportunityStore.List(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list opportunities: %v", err)
	}

	if len(allOpportunities) != 0 {
		t.Errorf("Expected empty store, found %d opportunities", len(allOpportunities))
	}
}

// TestFilterEligible verifies that filterEligible applies the isEligible gate correctly.
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
				makeOpp("sba-opp", "SBA"),   // eligible
				makeOpp("8a-opp", "8A"),     // ineligible
				makeOpp("open-opp", ""),     // eligible (full-and-open)
				makeOpp("wosb-opp", "WOSB"), // ineligible
				makeOpp("hub-opp", "HZC"),   // ineligible
			},
			wantEligible: 2,
			wantDropped:  3,
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
			},
			wantEligible: 0,
			wantDropped:  2,
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

// TestHunterIntegration_EligibilityGate tests that the cached fixture returns the
// correct eligible subset. Two of three fixture opportunities are eligible:
//   - a1b2c3d4e5f6 (SBA): eligible
//   - f6e5d4c3b2a1 (8A):  ineligible — dropped
//   - 9z8y7x6w5v4u (""): eligible (full-and-open)
func TestHunterIntegration_EligibilityGate(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	samClient, err := samgov.NewClient(samgov.Config{UseCached: true})
	if err != nil {
		t.Fatalf("Failed to create SAM.gov client: %v", err)
	}

	opportunityStore, err := store.NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSON store: %v", err)
	}

	all, err := samClient.FetchByNAICS(ctx, []string{"541512", "541519"})
	if err != nil {
		t.Fatalf("Failed to fetch opportunities: %v", err)
	}

	eligible, dropped := filterEligible(all)

	// Fixture has exactly one 8(a) opportunity — verify it was dropped
	if dropped != 1 {
		t.Errorf("Expected 1 dropped opportunity (8(a)), got %d", dropped)
	}
	if len(eligible) != len(all)-1 {
		t.Errorf("Expected %d eligible opportunities, got %d", len(all)-1, len(eligible))
	}

	for _, opp := range eligible {
		if err := opportunityStore.Save(ctx, opp); err != nil {
			t.Errorf("Failed to save opportunity %s: %v", opp.ID, err)
		}
	}

	saved, err := opportunityStore.List(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list saved opportunities: %v", err)
	}
	if len(saved) != len(eligible) {
		t.Errorf("Expected %d saved opportunities, got %d", len(eligible), len(saved))
	}

	for _, opp := range saved {
		if opp.SetAsideCode == "8A" {
			t.Errorf("Ineligible 8(a) opportunity %s was saved to store", opp.ID)
		}
	}

	t.Logf("Eligibility gate test: %d fetched, %d dropped, %d saved", len(all), dropped, len(saved))
}
