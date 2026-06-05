// Package main is the entry point for the Scorer agent.
//
// The Scorer reads unscored opportunities from the queue, scores each one for
// bid/no-bid fit against the company's Capability Profile, and writes the
// enriched opportunity back to the store.
//
// Zone 1 pipeline: Hunter → Scorer → Opportunity Queue (dashboard)
//
// Configuration via environment variables or flags:
//   - STORE_PATH: path to store directory (default: "./queue")
//   - PROFILE_PATH: path to capability_profile.json (default: "test/fixtures/capability_profile.json")
//
// Example usage:
//
//	# Score all unscored opportunities in ./queue
//	go run cmd/scorer/main.go
//
//	# Use a custom profile
//	go run cmd/scorer/main.go --profile=./profiles/bluemeta.json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Mawar2/Kaimi/internal/scorer"
	"github.com/Mawar2/Kaimi/internal/store"
)

// Config holds the Scorer agent configuration.
type Config struct {
	StorePath   string // Path to the store directory
	ProfilePath string // Path to capability_profile.json
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Scorer error: %v\n", err)
		os.Exit(1)
	}
}

// run contains the main logic for the Scorer agent.
func run() error {
	config := parseConfig()

	fmt.Println("Scorer agent starting...")
	fmt.Printf("Store path:   %s\n", config.StorePath)
	fmt.Printf("Profile path: %s\n", config.ProfilePath)

	// Load capability profile
	profile, err := loadProfile(config.ProfilePath)
	if err != nil {
		return fmt.Errorf("loading capability profile: %w", err)
	}
	fmt.Printf("Loaded profile for: %s\n", profile.CompanyName)

	// Initialize store
	opportunityStore, err := store.NewJSONStore(config.StorePath)
	if err != nil {
		return fmt.Errorf("initializing store: %w", err)
	}

	ctx := context.Background()

	// Initialize Gemini scorer
	s, err := scorer.NewGeminiScorer(ctx)
	if err != nil {
		return fmt.Errorf("initializing Gemini scorer: %w", err)
	}

	// List all opportunities
	fmt.Println("Listing opportunities...")
	all, err := opportunityStore.List(ctx, nil)
	if err != nil {
		return fmt.Errorf("listing opportunities: %w", err)
	}

	scoredCount := 0
	skippedCount := 0
	errorCount := 0
	startTime := time.Now()

	for _, opp := range all {
		// Skip already-scored opportunities
		if opp.ScoredAt != nil {
			skippedCount++
			continue
		}

		result, err := scorer.ScoreAndSave(ctx, s, opportunityStore, opp, profile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to score %s: %v\n", opp.ID, err)
			errorCount++
			continue
		}

		fmt.Printf("  ✓ %s: %d/100 [%s] — %s\n",
			opp.ID, result.Score, result.Recommendation, opp.Title)
		scoredCount++
	}

	fmt.Println("\n--- Scorer Summary ---")
	fmt.Printf("Total opportunities:  %d\n", len(all))
	fmt.Printf("Scored this run:      %d\n", scoredCount)
	fmt.Printf("Already scored:       %d\n", skippedCount)
	fmt.Printf("Errors:               %d\n", errorCount)
	fmt.Printf("Duration:             %v\n", time.Since(startTime))
	fmt.Println("\nScorer complete.")
	return nil
}

// loadProfile reads and parses the Capability Profile from a JSON file.
func loadProfile(path string) (*scorer.CapabilityProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading profile file %s: %w", path, err)
	}

	var profile scorer.CapabilityProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("parsing profile JSON: %w", err)
	}

	return &profile, nil
}

// parseConfig reads configuration from environment variables and flags.
func parseConfig() Config {
	storePath := flag.String("store-path", getEnv("STORE_PATH", "./queue"), "Store directory path")
	profilePath := flag.String("profile", getEnv("PROFILE_PATH", "test/fixtures/capability_profile.json"), "Capability profile JSON path")
	flag.Parse()

	return Config{
		StorePath:   *storePath,
		ProfilePath: *profilePath,
	}
}

// getEnv returns the value of an environment variable or a default.
func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
