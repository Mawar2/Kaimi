// Package main is the entry point for the Hunter agent.
//
// Hunter is the first agent in the Kaimi autonomous BD pipeline. It pulls federal
// contracting opportunities from the SAM.gov API, filters them by NAICS code, and
// saves them to the opportunity queue for downstream scoring.
//
// Configuration is read from environment variables or command-line flags:
//   - MODE: "cached" or "live" (default: cached)
//   - SAM_API_KEY: SAM.gov API key (required for live mode)
//   - PROFILE: Path to capability profile JSON or YAML (default: "./config/profile.json")
//   - STORE_TYPE: Store implementation type (default: "json")
//   - STORE_PATH: Path to store directory (default: "./queue")
//
// Example usage:
//
//	# Run in cached mode (for testing)
//	go run cmd/hunter/main.go --mode=cached
//
//	# Run in live mode with a custom profile
//	SAM_API_KEY=your-key go run cmd/hunter/main.go --mode=live --profile=./config/profile.json
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
	Mode        string // "cached" or "live"
	APIKey      string // SAM.gov API key (live mode only)
	ProfilePath string // Path to capability profile JSON or YAML
	StoreType   string // Store implementation type ("json")
	StorePath   string // Path to store directory
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Hunter error: %v\n", err)
		os.Exit(1)
	}
}

// run parses config and delegates to runWithConfig.
func run() error {
	config := parseConfig()
	if err := validateConfig(&config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	return runWithConfig(&config)
}

// runWithConfig contains the main logic for the Hunter agent and is extracted to enable
// test injection without flag parsing.
func runWithConfig(cfg *Config) error {
	// Load capability profile
	p, err := profile.LoadProfile(cfg.ProfilePath)
	if err != nil {
		return fmt.Errorf("failed to load capability profile: %w", err)
	}

	// Derive NAICS codes from profile
	naicsCodes := p.AllNAICSCodes()
	if len(naicsCodes) == 0 {
		return fmt.Errorf("capability profile has no NAICS codes configured")
	}

	// Log configuration (excluding sensitive data)
	fmt.Println("Hunter agent starting...")
	fmt.Printf("Mode:        %s\n", cfg.Mode)
	fmt.Printf("Profile:     %s\n", cfg.ProfilePath)
	fmt.Printf("NAICS codes: %v\n", naicsCodes)
	fmt.Printf("Store path:  %s\n", cfg.StorePath)

	// Initialize SAM.gov client
	samClient, err := samgov.NewClient(samgov.Config{
		APIKey:    cfg.APIKey,
		UseCached: cfg.Mode == "cached",
	})
	if err != nil {
		return fmt.Errorf("failed to create SAM.gov client: %w", err)
	}

	// Initialize store
	var opportunityStore store.Store
	switch cfg.StoreType {
	case "json":
		opportunityStore, err = store.NewJSONStore(cfg.StorePath)
		if err != nil {
			return fmt.Errorf("failed to create JSON store: %w", err)
		}
	default:
		return fmt.Errorf("unsupported store type: %s", cfg.StoreType)
	}

	// Fetch opportunities from SAM.gov
	ctx := context.Background()
	fmt.Println("Fetching opportunities from SAM.gov...")
	startTime := time.Now()

	opportunities, err := samClient.FetchByNAICS(ctx, naicsCodes)
	if err != nil {
		return fmt.Errorf("failed to fetch opportunities: %w", err)
	}

	fetchDuration := time.Since(startTime)
	fmt.Printf("Fetched %d opportunities in %v\n", len(opportunities), fetchDuration)

	// Apply eligibility gate: drop set-asides BlueMeta cannot compete on.
	fmt.Println("Applying eligibility gate...")
	eligible, dropped := filterEligible(opportunities)
	fmt.Printf("Eligibility gate: %d eligible, %d dropped (ineligible set-aside)\n", len(eligible), dropped)

	// Save eligible opportunities to store
	fmt.Println("Saving eligible opportunities to store...")
	savedCount := 0
	errorCount := 0

	for _, opp := range eligible {
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
	fmt.Printf("Opportunities fetched: %d\n", len(opportunities))
	fmt.Printf("Ineligible (dropped):  %d\n", dropped)
	fmt.Printf("Opportunities saved:   %d\n", savedCount)
	fmt.Printf("Errors:                %d\n", errorCount)
	fmt.Printf("Total duration:        %v\n", totalDuration)

	if errorCount > 0 {
		fmt.Printf("\nWarning: %d opportunities could not be saved\n", errorCount)
	}

	fmt.Println("\nHunter complete.")
	return nil
}

// parseConfig reads configuration from environment variables and command-line flags.
func parseConfig() Config {
	mode := flag.String("mode", getEnv("MODE", "cached"), "Mode: cached or live")
	profilePath := flag.String("profile", getEnv("PROFILE", "./config/profile.json"),
		"Path to capability profile JSON or YAML")
	storeType := flag.String("store-type", getEnv("STORE_TYPE", "json"), "Store type: json")
	storePath := flag.String("store-path", getEnv("STORE_PATH", "./queue"), "Store directory path")

	flag.Parse()

	// API key from environment only (never from command-line for security)
	apiKey := os.Getenv("SAM_API_KEY")

	return Config{
		Mode:        *mode,
		APIKey:      apiKey,
		ProfilePath: *profilePath,
		StoreType:   *storeType,
		StorePath:   *storePath,
	}
}

// validateConfig validates the configuration.
func validateConfig(config *Config) error {
	if config.Mode != "cached" && config.Mode != "live" {
		return fmt.Errorf("mode must be 'cached' or 'live', got: %s", config.Mode)
	}
	if config.Mode == "live" && config.APIKey == "" {
		return fmt.Errorf("SAM_API_KEY environment variable is required for live mode")
	}
	if config.ProfilePath == "" {
		return fmt.Errorf("profile path is required (--profile or PROFILE env var)")
	}
	if config.StoreType != "json" {
		return fmt.Errorf("unsupported store type: %s (only 'json' is supported in Phase 0)", config.StoreType)
	}
	return nil
}

// filterEligible applies the eligibility gate, returning opportunities BlueMeta can bid on
// and a count of those dropped.
func filterEligible(opportunities []*opportunity.Opportunity) (eligible []*opportunity.Opportunity, dropped int) {
	eligible = make([]*opportunity.Opportunity, 0, len(opportunities))
	for _, opp := range opportunities {
		if isEligible(opp.SetAsideCode) {
			eligible = append(eligible, opp)
		} else {
			fmt.Fprintf(os.Stderr, "Dropped ineligible opportunity %s (set-aside: %q)\n", opp.ID, opp.SetAsideCode)
			dropped++
		}
	}
	return eligible, dropped
}

// isEligible returns true if the SAM.gov set-aside code is one BlueMeta can legally compete on.
//
// Unrecognized codes pass through as eligible to avoid false negatives (conservative default).
// This mirrors profile.CapabilityProfile.IsEligible but operates on the raw code string,
// enabling filterEligible to work without a loaded profile at runtime.
func isEligible(code string) bool {
	code = strings.ToUpper(strings.TrimSpace(code))
	switch code {
	case "", "NONE":
		return true
	case "SBA", "SBP", "SDB":
		return true
	case "8A", "8(A)", "8AN":
		return false
	case "SDVOSB", "SDVOSBC", "SDVOSBS":
		return false
	case "WOSB", "WOSBSS", "EDWOSB", "EDWOSBSS":
		return false
	case "HUBZONE", "HUB", "HZC", "HZS":
		return false
	case "VOSB":
		return false
	case "IEE", "ISBEE":
		return false
	default:
		return true
	}
}

// getEnv returns the value of an environment variable or a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
