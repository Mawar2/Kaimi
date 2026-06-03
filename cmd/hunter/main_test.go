package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mawar2/Kaimi/internal/samgov"
	"github.com/Mawar2/Kaimi/internal/store"
)

// TestParseNAICSCodes verifies NAICS code parsing from comma-separated strings.
func TestParseNAICSCodes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single code",
			input:    "541512",
			expected: []string{"541512"},
		},
		{
			name:     "multiple codes",
			input:    "541512,541519,541330",
			expected: []string{"541512", "541519", "541330"},
		},
		{
			name:     "codes with spaces",
			input:    "541512, 541519, 541330",
			expected: []string{"541512", "541519", "541330"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "trailing comma",
			input:    "541512,541519,",
			expected: []string{"541512", "541519"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseNAICSCodes(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d codes, got %d", len(tt.expected), len(result))
				return
			}
			for i, code := range result {
				if code != tt.expected[i] {
					t.Errorf("Expected code %q at index %d, got %q", tt.expected[i], i, code)
				}
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
			err := validateConfig(tt.config)
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
	os.Setenv(testKey, testValue)
	defer os.Unsetenv(testKey)

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
