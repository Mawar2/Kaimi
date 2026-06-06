// Package main is the entry point for the Hunter agent.
//
// Hunter is the first agent in the Kaimi autonomous BD pipeline. It loads a
// CapabilityProfile from a JSON or YAML file, fetches federal contracting
// opportunities from the SAM.gov API by NAICS code, filters them through an
// eligibility gate, and saves eligible opportunities to the queue store.
//
// Configuration is read from environment variables or command-line flags:
//   - MODE: "cached" or "live" (default: cached)
//   - SAM_API_KEY: SAM.gov API key (required for live mode)
//   - PROFILE: Path to capability profile JSON or YAML (default: ./profile.json)
//   - STORE_TYPE: Store implementation type (default: "json")
//   - STORE_PATH: Path to store directory (default: "./queue")
//
// Example usage:
//
//	# Run in cached mode (for testing)
//	go run cmd/hunter/main.go --mode=cached
//
//	# Run in live mode with a custom profile
//	SAM_API_KEY=your-key go run cmd/hunter/main.go --mode=live --profile=./profile.json
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
	APIKey      string // SAM.gov API key
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

// run parses configuration and delegates to runWithConfig.
func run() error {
	config := parseConfig()
	return runWithConfig(&config)
}

// runWithConfig executes the full Hunter pipeline for the given configuration.
// It is separate from run() so tests can inject a Config without flag parsing.
func runWithConfig(config *Config) error {
	if err := validateConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Load capability profile to resolve NAICS codes.
	p, err := profile.LoadProfile(config.ProfilePath)
	if err != nil {
		return fmt.Errorf("failed to load profile: %w", err)
	}

	naicsCodes := p.AllNAICSCodes()
	if len(naicsCodes) == 0 {
		return fmt.Errorf("profile %q has no NAICS codes", config.ProfilePath)
	}

	fmt.Println("Hunter agent starting...")
	fmt.Printf("Mode:        %s\n", config.Mode)
	fmt.Printf("Profile:     %s (%d NAICS codes)\n", config.ProfilePath, len(naicsCodes))
	fmt.Printf("Store path:  %s\n", config.StorePath)

	// Initialize SAM.gov client.
	samClient, err := samgov.NewClient(samgov.Config{
		APIKey:    config.APIKey,
		UseCached: config.Mode == "cached",
	})
	if err != nil {
		return fmt.Errorf("failed to create SAM.gov client: %w", err)
	}

	// Initialize store.
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

	// Fetch opportunities from SAM.gov.
	ctx := context.Background()
	fmt.Println("Fetching opportunities from SAM.gov...")
	startTime := time.Now()

	opportunities, err := samClient.FetchByNAICS(ctx, naicsCodes)
	if err != nil {
		return fmt.Errorf("failed to fetch opportunities: %w", err)
	}

	fetchDuration := time.Since(startTime)
	fmt.Printf("Fetched %d opportunities in %v\n", len(opportunities), fetchDuration)

	// Apply eligibility gate.
	fmt.Println("Applying eligibility gate...")
	eligible, filtered := filterEligible(opportunities)
	fmt.Printf("Eligibility gate: %d eligible, %d dropped\n", len(eligible), filtered)

	// Save eligible opportunities to store.
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

	totalDuration := time.Since(startTime)
	fmt.Println("\n--- Hunter Summary ---")
	fmt.Printf("Opportunities fetched: %d\n", len(opportunities))
	fmt.Printf("Ineligible (dropped):  %d\n", filtered)
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
	profilePath := flag.String("profile", getEnv("PROFILE", "./profile.json"), "Path to capability profile (.json or .yaml)")
	storeType := flag.String("store-type", getEnv("STORE_TYPE", "json"), "Store type: json")
	storePath := flag.String("store-path", getEnv("STORE_PATH", "./queue"), "Store directory path")

	flag.Parse()

	// API key is never accepted on the command line.
	apiKey := os.Getenv("SAM_API_KEY")

	return Config{
		Mode:        *mode,
		APIKey:      apiKey,
		ProfilePath: *profilePath,
		StoreType:   *storeType,
		StorePath:   *storePath,
	}
}

// validateConfig checks that Config values are internally consistent.
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

// filterEligible applies the eligibility gate and returns the eligible subset
// along with a count of dropped opportunities.
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

// isEligible returns true if the SAM.gov set-aside code does not restrict competition
// to a program BlueMeta does not hold.
//
// Decision table:
//   - Empty / "NONE" / SBA / SBP — full-and-open or small business: eligible
//   - 8A / 8(A) / 8AN — 8(a) program not held: ineligible
//   - SDVOSB / SDVOSBC — service-disabled veteran certification not held: ineligible
//   - WOSB / EDWOSB — women-owned certification not held: ineligible
//   - HUBZONE / HUB — HUBZone certification not held: ineligible
//   - VOSB — veteran-owned certification not held: ineligible
//   - IEE / ISBEE — Indian enterprise certification not held: ineligible
//   - Unrecognized code — conservative passthrough to avoid false negatives: eligible
func isEligible(setAsideCode string) bool {
	switch strings.ToUpper(strings.TrimSpace(setAsideCode)) {
	case "", "NONE", "SBA", "SBP":
		return true
	case "8A", "8(A)", "8AN",
		"SDVOSB", "SDVOSBC",
		"WOSB", "EDWOSB",
		"HUBZONE", "HUB",
		"VOSB",
		"IEE", "ISBEE":
		return false
	default:
		// Unknown set-aside: pass through conservatively to avoid false negatives.
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
