// Package main is the entry point for the Hunter agent.
//
// Hunter is the first agent in the Kaimi autonomous BD pipeline. It pulls federal
// contracting opportunities from the SAM.gov API, gates them against BlueMeta's
// set-aside eligibility, and saves eligible opportunities to the queue for
// downstream scoring.
//
// Configuration is read from environment variables or command-line flags:
//   - MODE: "cached" or "live" (default: cached)
//   - SAM_API_KEY: SAM.gov API key (required for live mode)
//   - PROFILE: Path to capability profile JSON or YAML (default: "profile.json")
//   - STORE_TYPE: Store implementation type (default: "json")
//   - STORE_PATH: Path to store directory (default: "./queue")
//
// Example usage:
//
//	# Run in cached mode (for testing)
//	go run cmd/hunter/main.go --mode=cached
//
//	# Run in live mode with a custom profile
//	SAM_API_KEY=your-key go run cmd/hunter/main.go --mode=live --profile=profile.json
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Mawar2/Kaimi/internal/profile"
	"github.com/Mawar2/Kaimi/internal/samgov"
	"github.com/Mawar2/Kaimi/internal/store"
)

// Config holds the Hunter agent configuration.
type Config struct {
	Mode        string // "cached" or "live"
	APIKey      string // SAM.gov API key
	ProfilePath string // Path to capability profile JSON or YAML file
	StoreType   string // Store implementation type ("json")
	StorePath   string // Path to store directory
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Hunter error: %v\n", err)
		os.Exit(1)
	}
}

// run parses configuration, validates it, and delegates to runWithConfig.
func run() error {
	config := parseConfig()

	if err := validateConfig(&config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	ctx := context.Background()
	return runWithConfig(ctx, &config)
}

// runWithConfig loads the capability profile, fetches opportunities from SAM.gov,
// applies the set-aside eligibility gate, and saves eligible opportunities to the
// store. Extracted from run() so tests can inject a synthetic profile path and store path.
func runWithConfig(ctx context.Context, config *Config) error {
	prof, err := profile.LoadProfile(config.ProfilePath)
	if err != nil {
		return fmt.Errorf("failed to load capability profile: %w", err)
	}
	naicsCodes := prof.AllNAICSCodes()

	fmt.Println("Hunter agent starting...")
	fmt.Printf("Mode: %s\n", config.Mode)
	fmt.Printf("NAICS codes: %v\n", naicsCodes)
	fmt.Printf("Store path: %s\n", config.StorePath)

	samClient, err := samgov.NewClient(samgov.Config{
		APIKey:    config.APIKey,
		UseCached: config.Mode == "cached",
	})
	if err != nil {
		return fmt.Errorf("failed to create SAM.gov client: %w", err)
	}

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

	fmt.Println("Fetching opportunities from SAM.gov...")
	startTime := time.Now()

	opportunities, err := samClient.FetchByNAICS(ctx, naicsCodes)
	if err != nil {
		return fmt.Errorf("failed to fetch opportunities: %w", err)
	}

	fmt.Printf("Fetched %d opportunities in %v\n", len(opportunities), time.Since(startTime))

	savedCount := 0
	droppedCount := 0
	errorCount := 0

	for _, opp := range opportunities {
		if !isEligible(opp.SetAsideCode) {
			fmt.Fprintf(os.Stderr, "Dropped ineligible opportunity %s (set-aside: %q)\n", opp.ID, opp.SetAsideCode)
			droppedCount++
			continue
		}
		if saveErr := opportunityStore.Save(ctx, opp); saveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save opportunity %s: %v\n", opp.ID, saveErr)
			errorCount++
			continue
		}
		savedCount++
	}

	totalDuration := time.Since(startTime)
	fmt.Println("\n--- Hunter Summary ---")
	fmt.Printf("Opportunities fetched: %d\n", len(opportunities))
	fmt.Printf("Ineligible (dropped):  %d\n", droppedCount)
	fmt.Printf("Opportunities saved:   %d\n", savedCount)
	fmt.Printf("Errors:                %d\n", errorCount)
	fmt.Printf("Total duration:        %v\n", totalDuration)

	if errorCount > 0 {
		fmt.Printf("\nWarning: %d opportunities could not be saved\n", errorCount)
	}

	fmt.Println("\nHunter complete.")
	return nil
}

// isEligible reports whether BlueMeta can bid on an opportunity with the given
// set-aside code. It keeps full-and-open and small-business set-asides, drops
// program certifications BlueMeta does not hold, and passes unrecognized codes
// to avoid false negatives.
func isEligible(setAsideCode string) bool {
	switch strings.ToUpper(strings.TrimSpace(setAsideCode)) {
	case "", "NONE":
		// Full-and-open competition — always eligible
		return true
	case "SBA", "SBP":
		// Total/Partial Small Business Set-Aside — BlueMeta qualifies
		return true
	case "8A", "8(A)", "8AN",
		"SDVOSB", "SDVOSBC",
		"WOSB", "EDWOSB",
		"HUBZONE", "HUB",
		"VOSB",
		"IEE", "ISBEE":
		// Program certifications BlueMeta does not hold
		return false
	default:
		// Pass unrecognized codes to avoid false negatives
		return true
	}
}

// parseConfig reads configuration from environment variables and command-line flags.
func parseConfig() Config {
	mode := flag.String("mode", getEnv("MODE", "cached"), "Mode: cached or live")
	profilePath := flag.String("profile", getEnv("PROFILE", "profile.json"), "Path to capability profile JSON or YAML")
	storeType := flag.String("store-type", getEnv("STORE_TYPE", "json"), "Store type: json")
	storePath := flag.String("store-path", getEnv("STORE_PATH", "./queue"), "Store directory path")

	flag.Parse()

	// API key from environment only — never command-line for security
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
		return fmt.Errorf("profile path is required")
	}

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
