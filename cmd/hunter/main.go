// Package main is the entry point for the Hunter agent.
//
// Hunter is the first agent in the Kaimi autonomous BD pipeline. It pulls federal
// contracting opportunities from the SAM.gov API, filters them by NAICS code, and
// saves them to the opportunity queue for downstream scoring.
//
// Configuration is read from environment variables or command-line flags:
//   - MODE: "cached" or "live" (default: cached)
//   - SAM_API_KEY: SAM.gov API key (required for live mode)
//   - NAICS_CODES: Comma-separated list of NAICS codes (default: "541512,541519")
//   - STORE_TYPE: Store implementation type (default: "json")
//   - STORE_PATH: Path to store directory (default: "./queue")
//
// Example usage:
//
//	# Run in cached mode (for testing)
//	go run cmd/hunter/main.go --mode=cached
//
//	# Run in live mode
//	SAM_API_KEY=your-key go run cmd/hunter/main.go --mode=live --naics=541512,541519
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Mawar2/Kaimi/internal/samgov"
	"github.com/Mawar2/Kaimi/internal/store"
)

// Config holds the Hunter agent configuration.
type Config struct {
	Mode       string   // "cached" or "live"
	APIKey     string   // SAM.gov API key
	NAICSCodes []string // NAICS codes to search for
	StoreType  string   // Store implementation type ("json")
	StorePath  string   // Path to store directory
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

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Log configuration (excluding sensitive data)
	fmt.Println("Hunter agent starting...")
	fmt.Printf("Mode: %s\n", config.Mode)
	fmt.Printf("NAICS codes: %v\n", config.NAICSCodes)
	fmt.Printf("Store path: %s\n", config.StorePath)

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

	// Save opportunities to store
	fmt.Println("Saving opportunities to store...")
	savedCount := 0
	errorCount := 0

	for _, opp := range opportunities {
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
	// Define command-line flags
	mode := flag.String("mode", getEnv("MODE", "cached"), "Mode: cached or live")
	naicsStr := flag.String("naics", getEnv("NAICS_CODES", "541512,541519"), "Comma-separated NAICS codes")
	storeType := flag.String("store-type", getEnv("STORE_TYPE", "json"), "Store type: json")
	storePath := flag.String("store-path", getEnv("STORE_PATH", "./queue"), "Store directory path")

	flag.Parse()

	// Get API key from environment (never from command-line for security)
	apiKey := os.Getenv("SAM_API_KEY")

	// Parse NAICS codes
	naicsCodes := parseNAICSCodes(*naicsStr)

	return Config{
		Mode:       *mode,
		APIKey:     apiKey,
		NAICSCodes: naicsCodes,
		StoreType:  *storeType,
		StorePath:  *storePath,
	}
}

// validateConfig validates the configuration.
func validateConfig(config Config) error {
	// Validate mode
	if config.Mode != "cached" && config.Mode != "live" {
		return fmt.Errorf("mode must be 'cached' or 'live', got: %s", config.Mode)
	}

	// Validate API key for live mode
	if config.Mode == "live" && config.APIKey == "" {
		return fmt.Errorf("SAM_API_KEY environment variable is required for live mode")
	}

	// Validate NAICS codes
	if len(config.NAICSCodes) == 0 {
		return fmt.Errorf("at least one NAICS code is required")
	}

	// Validate store type
	if config.StoreType != "json" {
		return fmt.Errorf("unsupported store type: %s (only 'json' is supported in Phase 0)", config.StoreType)
	}

	return nil
}

// parseNAICSCodes parses a comma-separated string of NAICS codes.
func parseNAICSCodes(s string) []string {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	var codes []string
	for _, part := range parts {
		code := strings.TrimSpace(part)
		if code != "" {
			codes = append(codes, code)
		}
	}
	return codes
}

// getEnv returns the value of an environment variable or a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
