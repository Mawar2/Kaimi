// Package main is the entry point for the Hunter agent.
//
// Hunter is the first agent in the Kaimi autonomous BD pipeline. It pulls federal
// contracting opportunities from the SAM.gov API, filters them by NAICS code, and
// saves them to the opportunity queue for downstream scoring.
//
// Configuration is read from environment variables or command-line flags:
//   - MODE: "cached" or "live" (default: cached)
//   - SAM_API_KEY: SAM.gov API key (required for live mode)
//   - STORE_TYPE: Store implementation type (default: "json")
//   - STORE_PATH: Path to store directory (default: "./queue")
//   - PROFILE_PATH: Path to capability profile JSON/YAML (default: "profile.json")
//
// Example usage:
//
//	# Run in cached mode (for testing)
//	go run cmd/hunter/main.go --mode=cached
//
//	# Run in live mode
//	SAM_API_KEY=your-key go run cmd/hunter/main.go --mode=live
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/profile"
	"github.com/Mawar2/Kaimi/internal/samgov"
	"github.com/Mawar2/Kaimi/internal/store"
)

// Config holds the Hunter agent configuration.
type Config struct {
	Mode        string   // "cached" or "live"
	APIKey      string   // SAM.gov API key
	NAICSCodes  []string // NAICS codes to search for (loaded from capability profile)
	StoreType   string   // Store implementation type ("json")
	StorePath   string   // Path to store directory
	ProfilePath string   // Path to the Capability Profile config file
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Hunter error: %v\n", err)
		os.Exit(1)
	}
}

// run contains the main logic for the Hunter agent.
func run() error {
	// Parse configuration
	config := parseConfig()
	return runWithConfig(&config)
}

// runWithConfig executes the Hunter agent workflow with the given configuration.
func runWithConfig(config *Config) error {
	// Validate configuration
	if err := validateConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Load capability profile
	prof, err := profile.LoadProfile(config.ProfilePath)
	if err != nil {
		return fmt.Errorf("failed to load capability profile: %w", err)
	}

	// Use NAICS codes from profile
	var naicsCodes []string
	seenCodes := make(map[string]bool)
	addCodes := func(codes []string) {
		for _, code := range codes {
			code = strings.TrimSpace(code)
			if code != "" && !seenCodes[code] {
				seenCodes[code] = true
				naicsCodes = append(naicsCodes, code)
			}
		}
	}
	addCodes(prof.NAICS.Primary)
	addCodes(prof.NAICS.Secondary)
	addCodes(prof.NAICS.Tertiary)

	if len(naicsCodes) == 0 {
		return fmt.Errorf("no NAICS codes found in capability profile")
	}
	config.NAICSCodes = naicsCodes

	// Log configuration (excluding sensitive data)
	fmt.Println("Hunter agent starting...")
	fmt.Printf("Mode: %s\n", config.Mode)
	fmt.Printf("NAICS codes: %v\n", config.NAICSCodes)
	fmt.Printf("Store path: %s\n", config.StorePath)
	fmt.Printf("Profile path: %s\n", config.ProfilePath)

	// Initialize SAM.gov client
	samClient, err := samgov.NewClient(samgov.Config{
		APIKey:    config.APIKey,
		UseCached: config.Mode == "cached",
	})
	if err != nil {
		return fmt.Errorf("failed to create SAM.gov client: %w", err)
	}

	// Initialize Store
	var opportunityStore store.Store
	switch config.StoreType {
	case "json":
		opportunityStore, err = store.NewJSONStore(config.StorePath)
		if err != nil {
			return fmt.Errorf("failed to create JSON store: %w", err)
		}
	default:
		return fmt.Errorf("unsupported store type: %s", config.StoreType)
	}

	// Fetch opportunities from SAM.gov
	ctx := context.Background()
	fmt.Println("Fetching opportunities from SAM.gov...")
	startTime := time.Now()

	opportunities, err := samClient.FetchByNAICS(ctx, config.NAICSCodes)
	if err != nil {
		return fmt.Errorf("failed to fetch opportunities: %w", err)
	}

	fetchDuration := time.Since(startTime)
	fmt.Printf("Fetched %d opportunities in %v\n", len(opportunities), fetchDuration)

	// Filter opportunities by set-aside eligibility
	var eligibleOpportunities []*opportunity.Opportunity
	for _, opp := range opportunities {
		if isEligible(opp, prof) {
			eligibleOpportunities = append(eligibleOpportunities, opp)
		} else {
			fmt.Printf("Dropping ineligible opportunity %s (Title: %q, Set-Aside: %s)\n", opp.ID, opp.Title, opp.SetAsideCode)
		}
	}
	fmt.Printf("Filtered to %d eligible opportunities\n", len(eligibleOpportunities))

	// Save opportunities to store
	fmt.Println("Saving opportunities to store...")
	savedCount := 0
	errorCount := 0

	for _, opp := range eligibleOpportunities {
		if err := opportunityStore.Save(ctx, opp); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save opportunity %s: %v\n", opp.ID, err)
			errorCount++
			continue
		}
		savedCount++
	}

	// Log summary
	totalDuration := time.Since(startTime)
	fmt.Println("\n--- Hunter Summary ---")
	fmt.Printf("Opportunities fetched:  %d\n", len(opportunities))
	fmt.Printf("Opportunities eligible: %d\n", len(eligibleOpportunities))
	fmt.Printf("Opportunities saved:    %d\n", savedCount)
	fmt.Printf("Errors:                 %d\n", errorCount)
	fmt.Printf("Total duration:         %v\n", totalDuration)

	if errorCount > 0 {
		fmt.Printf("\nWarning: %d opportunities could not be saved\n", errorCount)
	}

	fmt.Println("\nHunter complete.")
	return nil
}

// isEligible determines if an opportunity is eligible based on its set-aside type and our Capability Profile.
func isEligible(opp *opportunity.Opportunity, prof *profile.CapabilityProfile) bool {
	// If set-aside is empty or none, it's full-and-open, which is eligible
	code := strings.ToUpper(strings.TrimSpace(opp.SetAsideCode))
	if code == "" || code == "NONE" {
		return true
	}

	// If it's a general small business set-aside (SBA or SBP), it is eligible since we are a small business
	if code == "SBA" || code == "SBP" {
		return true
	}

	// Drop ones set aside for programs we don't hold (8(a), SDVOSB, WOSB, HUBZone, VOSB, IEE, ISBEE, etc.)
	// BlueMeta only has small_business, sdb, minority_owned.
	// We drop:
	// - 8(a) -> "8A", "8(A)", "8AN"
	// - SDVOSB -> "SDVOSB", "SDVOSBC", "SDVOSBS", "SDVOS"
	// - WOSB -> "WOSB", "WOSBSS", "EDWOSB", "EDWOSBSS"
	// - HUBZone -> "HUBZONE", "HUB", "HS3", "HCS"
	// - VOSB -> "VOSB", "VOSBSS"
	// - IEE / ISBEE
	switch code {
	case "8A", "8(A)", "8AN",
		"SDVOSB", "SDVOSBC", "SDVOSBS", "SDVOS",
		"WOSB", "WOSBSS", "EDWOSB", "EDWOSBSS",
		"HUBZONE", "HUB", "HS3", "HCS",
		"VOSB", "VOSBSS",
		"IEE", "ISBEE":
		return false
	}

	// SDB / minority owned are NOT hard filters here. If SAM.gov returned SDB or minority owned set-aside
	// (though usually not separate codes), we keep them.
	// For any other unrecognized set-asides, we keep them to avoid starving the pipeline.
	return true
}

// parseConfig reads configuration from environment variables and command-line flags.
func parseConfig() Config {
	// Define command-line flags
	mode := flag.String("mode", getEnv("MODE", "cached"), "Mode: cached or live")
	storeType := flag.String("store-type", getEnv("STORE_TYPE", "json"), "Store type: json")
	storePath := flag.String("store-path", getEnv("STORE_PATH", "./queue"), "Store directory path")
	profilePath := flag.String("profile", getEnv("PROFILE_PATH", "profile.json"), "Path to capability profile JSON/YAML")

	flag.Parse()

	// Get API key from environment (never from command-line for security)
	apiKey := os.Getenv("SAM_API_KEY")

	return Config{
		Mode:        *mode,
		APIKey:      apiKey,
		StoreType:   *storeType,
		StorePath:   *storePath,
		ProfilePath: *profilePath,
	}
}

// validateConfig validates the configuration.
func validateConfig(config *Config) error {
	// Validate mode
	if config.Mode != "cached" && config.Mode != "live" {
		return fmt.Errorf("mode must be 'cached' or 'live', got: %s", config.Mode)
	}

	// Validate API key for live mode
	if config.Mode == "live" && config.APIKey == "" {
		return fmt.Errorf("SAM_API_KEY environment variable is required for live mode")
	}

	// Default ProfilePath if empty
	if config.ProfilePath == "" {
		config.ProfilePath = "profile.json"
	}

	// Validate store type
	if config.StoreType != "json" {
		return fmt.Errorf("unsupported store type: %s (only 'json' is supported in Phase 0)", config.StoreType)
	}

	return nil
}

// getEnv returns the value of an environment variable or a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
