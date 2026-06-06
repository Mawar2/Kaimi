// Package main is the entry point for the Hunter agent.
//
// Hunter is the first agent in the Kaimi autonomous BD pipeline. It pulls federal
// contracting opportunities from the SAM.gov API, filters them by NAICS code, and
// saves them to the opportunity queue for downstream scoring.
//
// Configuration is read from environment variables or command-line flags:
//   - MODE: "cached" or "live" (default: cached)
//   - SAM_API_KEY: SAM.gov API key (required for live mode)
//   - PROFILE_PATH: path to JSON or YAML capability profile (default: embedded BlueMeta)
//   - STORE_TYPE: Store implementation type (default: "json")
//   - STORE_PATH: Path to store directory (default: "./queue")
//
// Example usage:
//
//	# Run in cached mode using embedded profile
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

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/profile"
	"github.com/Mawar2/Kaimi/internal/samgov"
	"github.com/Mawar2/Kaimi/internal/store"
)

// Config holds the Hunter agent configuration.
type Config struct {
	Mode        string   // "cached" or "live"
	APIKey      string   // SAM.gov API key
	ProfilePath string   // path to capability profile JSON/YAML (empty = use embedded BlueMeta)
	NAICSCodes  []string // NAICS codes to search; populated from profile when empty
	StoreType   string   // Store implementation type ("json")
	StorePath   string   // Path to store directory
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Hunter error: %v\n", err)
		os.Exit(1)
	}
}

// run parses configuration and delegates to runWithConfig.
func run() error {
	cfg := parseConfig()
	return runWithConfig(&cfg)
}

// runWithConfig runs the full Hunter pipeline with the given configuration.
// It is extracted from run() to enable test injection without flag parsing.
func runWithConfig(cfg *Config) error {
	// Load capability profile: file if specified, otherwise embedded default.
	var prof *profile.CapabilityProfile
	if cfg.ProfilePath != "" {
		var err error
		prof, err = profile.LoadProfile(cfg.ProfilePath)
		if err != nil {
			return fmt.Errorf("failed to load profile: %w", err)
		}
	} else {
		prof = profile.BlueMeta
	}

	// Derive NAICS codes from profile when not explicitly overridden.
	if len(cfg.NAICSCodes) == 0 {
		cfg.NAICSCodes = prof.AllNAICSCodes()
	}

	// Validate configuration after NAICS codes are resolved.
	if err := validateConfig(cfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	fmt.Println("Hunter agent starting...")
	fmt.Printf("Mode: %s\n", cfg.Mode)
	fmt.Printf("NAICS codes: %v\n", cfg.NAICSCodes)
	fmt.Printf("Store path: %s\n", cfg.StorePath)

	// Initialize SAM.gov client.
	samClient, err := samgov.NewClient(samgov.Config{
		APIKey:    cfg.APIKey,
		UseCached: cfg.Mode == "cached",
	})
	if err != nil {
		return fmt.Errorf("failed to create SAM.gov client: %w", err)
	}

	// Initialize Store.
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

	// Fetch opportunities from SAM.gov.
	ctx := context.Background()
	fmt.Println("Fetching opportunities from SAM.gov...")
	startTime := time.Now()

	opportunities, err := samClient.FetchByNAICS(ctx, cfg.NAICSCodes)
	if err != nil {
		return fmt.Errorf("failed to fetch opportunities: %w", err)
	}

	fetchDuration := time.Since(startTime)
	fmt.Printf("Fetched %d opportunities in %v\n", len(opportunities), fetchDuration)

	// Apply eligibility gate: drop set-asides BlueMeta is not eligible for.
	fmt.Println("Applying eligibility gate...")
	eligible, filtered := filterEligible(opportunities)
	fmt.Printf("Eligibility gate: %d eligible, %d dropped (ineligible set-aside)\n",
		len(eligible), filtered)

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
	profilePath := flag.String("profile", getEnv("PROFILE_PATH", ""),
		"Path to capability profile JSON or YAML file (default: embedded BlueMeta profile)")
	storeType := flag.String("store-type", getEnv("STORE_TYPE", "json"), "Store type: json")
	storePath := flag.String("store-path", getEnv("STORE_PATH", "./queue"), "Store directory path")

	flag.Parse()

	// API key is always from environment for security.
	apiKey := os.Getenv("SAM_API_KEY")

	return Config{
		Mode:        *mode,
		APIKey:      apiKey,
		ProfilePath: *profilePath,
		StoreType:   *storeType,
		StorePath:   *storePath,
	}
}

// validateConfig validates the configuration. NAICSCodes must be populated
// before calling (runWithConfig derives them from the profile first).
func validateConfig(cfg *Config) error {
	if cfg.Mode != "cached" && cfg.Mode != "live" {
		return fmt.Errorf("mode must be 'cached' or 'live', got: %s", cfg.Mode)
	}

	if cfg.Mode == "live" && cfg.APIKey == "" {
		return fmt.Errorf("SAM_API_KEY environment variable is required for live mode")
	}

	if len(cfg.NAICSCodes) == 0 {
		return fmt.Errorf("at least one NAICS code is required")
	}

	if cfg.StoreType != "json" {
		return fmt.Errorf("unsupported store type: %s (only 'json' is supported in Phase 0)",
			cfg.StoreType)
	}

	return nil
}

// filterEligible applies the eligibility gate, returning the subset of opportunities
// that pass along with a count of those dropped.
func filterEligible(opportunities []*opportunity.Opportunity) (eligible []*opportunity.Opportunity, dropped int) {
	eligible = make([]*opportunity.Opportunity, 0, len(opportunities))
	for _, opp := range opportunities {
		if isEligible(opp.SetAsideCode) {
			eligible = append(eligible, opp)
		} else {
			fmt.Fprintf(os.Stderr, "Dropped ineligible opportunity %s (set-aside: %q)\n",
				opp.ID, opp.SetAsideCode)
			dropped++
		}
	}
	return eligible, dropped
}

// isEligible reports whether a SAM.gov set-aside code is eligible for BlueMeta.
//
// The switch covers all known set-aside families. Unrecognized codes pass through
// to avoid false negatives — it is better to score an ineligible opportunity than
// to silently drop an eligible one.
//
// Decision table:
//
//	""/"NONE"           → eligible  (full-and-open)
//	SBA/SBP             → eligible  (small business; BlueMeta qualifies)
//	8A/8(A)/8AN         → ineligible (8(a) cert not held)
//	SDVOSB/SDVOSBC      → ineligible (SDVOSB cert not held)
//	WOSB/EDWOSB         → ineligible (WOSB cert not held)
//	HUBZONE/HUB         → ineligible (HUBZone cert not held)
//	VOSB                → ineligible (VOSB cert not held)
//	IEE/ISBEE           → ineligible (Indian enterprise cert not held)
//	unrecognized        → eligible  (conservative passthrough)
func isEligible(setAsideCode string) bool {
	code := strings.ToUpper(strings.TrimSpace(setAsideCode))
	switch code {
	case "", "NONE":
		return true
	case "SBA", "SBP":
		return true
	case "8A", "8(A)", "8AN":
		return false
	case "SDVOSB", "SDVOSBC", "SDVOSBS":
		return false
	case "WOSB", "EDWOSB", "WOSBSS", "EDWOSBSS":
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
