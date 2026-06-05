package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/profile"
	"github.com/Mawar2/Kaimi/internal/samgov"
	"github.com/Mawar2/Kaimi/internal/store"
)

// TestIsEligible verifies set-aside eligibility gating.
func TestIsEligible(t *testing.T) {
	prof := &profile.CapabilityProfile{
		SetAside: profile.SetAsideStatus{
			SmallBusiness: true,
			SDB:           true,
			MinorityOwned: true,
		},
	}

	tests := []struct {
		name         string
		setAsideCode string
		expected     bool
	}{
		{"empty set-aside (full and open)", "", true},
		{"NONE set-aside (full and open)", "NONE", true},
		{"SBA set-aside (small business)", "SBA", true},
		{"SBP set-aside (small business)", "SBP", true},
		{"8(a) set-aside (not held)", "8A", false},
		{"8(a) alternate spelling (not held)", "8(A)", false},
		{"8(a) AN set-aside (not held)", "8AN", false},
		{"SDVOSB set-aside (not held)", "SDVOSB", false},
		{"SDVOSBC set-aside (not held)", "SDVOSBC", false},
		{"WOSB set-aside (not held)", "WOSB", false},
		{"EDWOSB set-aside (not held)", "EDWOSB", false},
		{"HUBZone set-aside (not held)", "HUBZONE", false},
		{"HUB set-aside (not held)", "HUB", false},
		{"VOSB set-aside (not held)", "VOSB", false},
		{"IEE set-aside (not held)", "IEE", false},
		{"ISBEE set-aside (not held)", "ISBEE", false},
		{"unrecognized set-aside (keep to avoid starving)", "OTHER_NEW", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opp := &opportunity.Opportunity{
				SetAsideCode: tt.setAsideCode,
			}
			result := isEligible(opp, prof)
			if result != tt.expected {
				t.Errorf("isEligible(%q) = %t; want %t", tt.setAsideCode, result, tt.expected)
			}
		})
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
				Mode:      "cached",
				StoreType: "json",
				StorePath: "./queue",
			},
			shouldError: false,
		},
		{
			name: "valid live config with API key",
			config: Config{
				Mode:      "live",
				APIKey:    "test-api-key",
				StoreType: "json",
				StorePath: "./queue",
			},
			shouldError: false,
		},
		{
			name: "invalid mode",
			config: Config{
				Mode:      "invalid",
				StoreType: "json",
			},
			shouldError: true,
		},
		{
			name: "live mode without API key",
			config: Config{
				Mode:      "live",
				StoreType: "json",
			},
			shouldError: true,
		},
		{
			name: "unsupported store type",
			config: Config{
				Mode:      "cached",
				StoreType: "firestore",
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
	// Set a test environment variable
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

// TestRunWithConfig verifies the full Hunter execution flow using a test profile.
func TestRunWithConfig(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test profile JSON file
	profilePath := filepath.Join(tempDir, "test_profile.json")
	testProfile := `{
		"uei": "TESTUEI",
		"cage": "TESTCAGE",
		"naics": {
			"primary": ["541512"],
			"secondary": ["541519"],
			"tertiary": []
		},
		"set_aside": {
			"small_business": true,
			"sdb": false,
			"minority_owned": false
		}
	}`

	if err := os.WriteFile(profilePath, []byte(testProfile), 0o644); err != nil {
		t.Fatalf("Failed to write test profile: %v", err)
	}

	config := Config{
		Mode:        "cached",
		StoreType:   "json",
		StorePath:   tempDir,
		ProfilePath: profilePath,
	}

	err := runWithConfig(&config)
	if err != nil {
		t.Fatalf("runWithConfig failed: %v", err)
	}

	// Verify that eligible opportunities were saved to the store.
	storePath := filepath.Join(tempDir, "queue")
	entries, err := os.ReadDir(storePath)
	if err != nil {
		t.Fatalf("Failed to read store queue directory: %v", err)
	}

	if len(entries) == 0 {
		t.Error("Expected opportunities to be saved to store, but store is empty")
	}
}

// TestHunterIntegration is an end-to-end integration test for the Hunter agent.
//
// This test runs the complete Hunter workflow in cached mode:
// 1. Initialize SAM.gov client in cached mode
// 2. Initialize JSON store
// 3. Fetch opportunities from cached fixtures
// 4. Save opportunities to store
// 5. Verify opportunities were saved correctly
func TestHunterIntegration(t *testing.T) {
	ctx := context.Background()

	// Create temporary directory for store
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "queue")

	// Initialize SAM.gov client in cached mode
	samClient, err := samgov.NewClient(samgov.Config{
		UseCached: true,
	})
	if err != nil {
		t.Fatalf("Failed to create SAM.gov client: %v", err)
	}

	// Initialize JSON store
	opportunityStore, err := store.NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSON store: %v", err)
	}

	// Fetch opportunities
	naicsCodes := []string{"541512", "541519"}
	opportunities, err := samClient.FetchByNAICS(ctx, naicsCodes)
	if err != nil {
		t.Fatalf("Failed to fetch opportunities: %v", err)
	}

	// Verify we got opportunities
	if len(opportunities) == 0 {
		t.Fatal("Expected to fetch at least one opportunity")
	}

	t.Logf("Fetched %d opportunities", len(opportunities))

	// Save opportunities to store
	savedCount := 0
	for _, opp := range opportunities {
		if err := opportunityStore.Save(ctx, opp); err != nil {
			t.Errorf("Failed to save opportunity %s: %v", opp.ID, err)
			continue
		}
		savedCount++
	}

	// Verify all opportunities were saved
	if savedCount != len(opportunities) {
		t.Errorf("Expected to save %d opportunities, saved %d", len(opportunities), savedCount)
	}

	// Verify opportunities can be retrieved from store
	for _, opp := range opportunities {
		retrieved, err := opportunityStore.Get(ctx, opp.ID)
		if err != nil {
			t.Errorf("Failed to retrieve opportunity %s: %v", opp.ID, err)
			continue
		}

		// Verify key fields match
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

	// Verify JSON files were created
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

	// Create temporary directory for store
	tempDir := t.TempDir()

	// Initialize SAM.gov client in cached mode
	samClient, err := samgov.NewClient(samgov.Config{
		UseCached: true,
	})
	if err != nil {
		t.Fatalf("Failed to create SAM.gov client: %v", err)
	}

	// Initialize JSON store
	opportunityStore, err := store.NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create JSON store: %v", err)
	}

	// Fetch opportunities with non-matching NAICS code
	naicsCodes := []string{"999999"} // This NAICS code doesn't exist in fixtures
	opportunities, err := samClient.FetchByNAICS(ctx, naicsCodes)
	if err != nil {
		t.Fatalf("Failed to fetch opportunities: %v", err)
	}

	// Verify no opportunities were found
	if len(opportunities) != 0 {
		t.Errorf("Expected 0 opportunities for non-matching NAICS, got %d", len(opportunities))
	}

	// Verify store is empty
	allOpportunities, err := opportunityStore.List(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list opportunities: %v", err)
	}

	if len(allOpportunities) != 0 {
		t.Errorf("Expected empty store, found %d opportunities", len(allOpportunities))
	}
}
