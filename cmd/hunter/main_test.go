package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/store"
)

// TestValidateConfig verifies configuration validation with the profile-based Config.
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
				ProfilePath: "./config/profile.json",
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
				ProfilePath: "./config/profile.json",
				StoreType:   "json",
				StorePath:   "./queue",
			},
			shouldError: false,
		},
		{
			name: "invalid mode",
			config: Config{
				Mode:        "invalid",
				ProfilePath: "./config/profile.json",
				StoreType:   "json",
			},
			shouldError: true,
		},
		{
			name: "live mode without API key",
			config: Config{
				Mode:        "live",
				ProfilePath: "./config/profile.json",
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
				ProfilePath: "./config/profile.json",
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

// TestFilterEligible verifies that filterEligible applies the built-in eligibility switch.
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
				makeOpp("hub-opp", "HZC"),   // ineligible (legacy HUBZone)
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

// TestRunWithConfig tests the full runWithConfig workflow in cached mode.
//
// Uses a minimal inline profile with NAICS codes matching the SAM.gov fixture.
// Fixture opportunities: 3 fetched, 1 dropped (8A set-aside), 2 saved.
func TestRunWithConfig(t *testing.T) {
	// Write a minimal profile to a temp file so we control NAICS codes.
	// Codes 541512 and 541519 match the samgov_response.json fixture.
	profileData := map[string]interface{}{
		"company": "Test Company",
		"naics_codes": []map[string]string{
			{"code": "541512", "description": "Computer Systems Design Services", "tier": "primary"},
			{"code": "541519", "description": "Other Computer Related Services", "tier": "primary"},
		},
	}
	profileBytes, err := json.Marshal(profileData)
	if err != nil {
		t.Fatalf("Failed to marshal test profile: %v", err)
	}

	profileFile := filepath.Join(t.TempDir(), "test_profile.json")
	if err := os.WriteFile(profileFile, profileBytes, 0o600); err != nil {
		t.Fatalf("Failed to write test profile: %v", err)
	}

	tempDir := t.TempDir()

	cfg := &Config{
		Mode:        "cached",
		ProfilePath: profileFile,
		StoreType:   "json",
		StorePath:   tempDir,
	}

	if err := runWithConfig(cfg); err != nil {
		t.Fatalf("runWithConfig error: %v", err)
	}

	// Verify store contents
	ctx := context.Background()
	s, err := store.NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("store.NewJSONStore error: %v", err)
	}

	saved, err := s.List(ctx, nil)
	if err != nil {
		t.Fatalf("store.List error: %v", err)
	}

	// Fixture: 3 total, 1 dropped (8A), 2 saved
	if len(saved) != 2 {
		t.Errorf("Expected 2 saved opportunities, got %d", len(saved))
	}

	// Verify the 8(a) opportunity was NOT saved
	for _, opp := range saved {
		if strings.ToUpper(strings.TrimSpace(opp.SetAsideCode)) == "8A" {
			t.Errorf("8(a) opportunity %s should not have been saved", opp.ID)
		}
	}
}
