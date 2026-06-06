// Package main is the entry point for the outline-probe developer diagnostic tool.
//
// outline-probe manually exercises the Outline agent against real or cached SAM.gov
// data. It is NOT part of the production pipeline and has no unit tests.
//
// Three modes:
//   - cached (default): uses fixture data, no API key needed
//   - live: searches real SAM.gov, requires SAM_API_KEY, respects --limit
//   - --pdf-file: extracts text from a local PDF via pdftotext, bypasses SAM.gov
//
// Usage:
//
//	# Cached mode — no API key needed
//	go run ./cmd/outline-probe
//
//	# Live mode — searches SAM.gov, limited to N opportunities
//	SAM_API_KEY=your-key go run ./cmd/outline-probe --mode=live --limit=3
//
//	# Local PDF — extract text from a file on disk
//	go run ./cmd/outline-probe --pdf-file=path/to/solicitation.pdf
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/outline"
	"github.com/Mawar2/Kaimi/internal/profile"
	"github.com/Mawar2/Kaimi/internal/samgov"
)

const separator = "═══════════════════════════════════════════════════════════"

// Config holds outline-probe configuration.
type Config struct {
	Mode    string // "cached" or "live"
	APIKey  string // SAM.gov API key (live mode only)
	Limit   int    // max opportunities to process (live mode, 0 = no limit)
	PDFFile string // path to local PDF file (pdf-file mode)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "outline-probe error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	config := parseConfig()

	// pdf-file mode bypasses SAM.gov entirely.
	if config.PDFFile != "" {
		return runPDFMode(config.PDFFile)
	}

	if err := validateConfig(&config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	fmt.Printf("outline-probe starting (mode: %s)\n", config.Mode)
	if config.Limit > 0 {
		fmt.Printf("Limit: %d opportunities\n", config.Limit)
	}
	fmt.Println()

	samClient, err := samgov.NewClient(samgov.Config{
		APIKey:    config.APIKey,
		UseCached: config.Mode == "cached",
	})
	if err != nil {
		return fmt.Errorf("failed to create SAM.gov client: %w", err)
	}

	ctx := context.Background()
	fmt.Printf("Fetching opportunities for %d NAICS codes...\n\n", len(profile.BlueMeta.NAICSCodes))

	opportunities, err := samClient.FetchByNAICS(ctx, profile.BlueMeta.NAICSCodes)
	if err != nil {
		return fmt.Errorf("failed to fetch opportunities: %w", err)
	}

	// Cap results in live mode when --limit is set.
	if config.Limit > 0 && len(opportunities) > config.Limit {
		opportunities = opportunities[:config.Limit]
	}

	if len(opportunities) == 0 {
		fmt.Println("No opportunities found.")
		return nil
	}

	outlineAgent := outline.New()
	successCount := 0
	errorCount := 0

	for i, opp := range opportunities {
		fmt.Println(separator)
		setAside := "(none)"
		if opp.SetAsideCode != "" {
			setAside = fmt.Sprintf("%q", opp.SetAsideCode)
		}
		fmt.Printf("[%d/%d] %s\n", i+1, len(opportunities), opp.Title)
		fmt.Printf("      ID: %s | NAICS: %s | Set-Aside: %s\n\n", opp.ID, opp.NAICSCode, setAside)

		if err := processOpportunity(ctx, outlineAgent, opp); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			errorCount++
		} else {
			successCount++
		}

		fmt.Println()
	}

	fmt.Println(separator)
	fmt.Printf("\nSummary: %d processed (%d errors)\n", successCount+errorCount, errorCount)
	fmt.Println("outline-probe complete.")
	return nil
}

// processOpportunity runs the Outline agent on one opportunity and prints the result.
func processOpportunity(ctx context.Context, a *outline.Agent, opp *opportunity.Opportunity) error {
	outlineResult, agentResult, err := a.Run(ctx, opp)
	if err != nil {
		return fmt.Errorf("outline agent failed for %s: %w", opp.ID, err)
	}

	fmt.Printf("Status:  %s\n", agentResult.Status)
	fmt.Printf("Summary: %s\n\n", agentResult.Summary)

	fmt.Printf("Sections (%d):\n", len(outlineResult.Sections))
	for _, s := range outlineResult.Sections {
		fmt.Printf("  %-40s %s\n", s.Title, s.Rationale)
	}

	fmt.Println("\nFormatting Rules:")
	printRule("Page Limit:", outlineResult.FormattingRules.PageLimit)
	printRule("Font:", outlineResult.FormattingRules.Font)
	printRule("Margins:", outlineResult.FormattingRules.Margins)
	printRule("Line Spacing:", outlineResult.FormattingRules.LineSpacing)
	printRule("File Format:", outlineResult.FormattingRules.FileFormat)
	if len(outlineResult.FormattingRules.RequiredForms) > 0 {
		fmt.Printf("  %-16s %s\n", "Required Forms:", strings.Join(outlineResult.FormattingRules.RequiredForms, ", "))
	}

	return nil
}

// printRule prints one formatting rule with consistent column alignment.
func printRule(label string, rule *outline.FormattingRule) {
	if rule.Specified {
		fmt.Printf("  %-16s %s\n", label, rule.Value)
	} else {
		fmt.Printf("  %-16s (not specified in solicitation)\n", label)
	}
}

// runPDFMode extracts text from a local PDF and runs the Outline agent on it.
func runPDFMode(pdfPath string) error {
	fmt.Printf("outline-probe starting (mode: pdf-file)\n")
	fmt.Printf("PDF: %s\n\n", pdfPath)

	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		return fmt.Errorf("PDF file not found: %s", pdfPath)
	}

	// pdftotext is provided by poppler-utils; dash sends output to stdout.
	cmd := exec.Command("pdftotext", pdfPath, "-")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("pdftotext failed (is poppler-utils installed?): %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return fmt.Errorf("pdftotext not found — install poppler-utils: %w", err)
	}

	text := strings.TrimSpace(string(output))
	if text == "" {
		return fmt.Errorf("pdftotext produced no text from %s", pdfPath)
	}

	fmt.Printf("Extracted %d characters from PDF\n\n", len(text))

	opp := &opportunity.Opportunity{
		ID:          "pdf-probe",
		Title:       pdfPath,
		Description: text,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	ctx := context.Background()
	a := outline.New()

	fmt.Println(separator)
	fmt.Printf("File: %s\n\n", pdfPath)

	if err := processOpportunity(ctx, a, opp); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(separator)
	fmt.Println("\noutline-probe complete.")
	return nil
}

// parseConfig reads configuration from flags and environment variables.
func parseConfig() Config {
	mode := flag.String("mode", getEnv("MODE", "cached"), "Mode: cached or live")
	limit := flag.Int("limit", 0, "Max opportunities to process in live mode (0 = no limit)")
	pdfFile := flag.String("pdf-file", "", "Path to a local PDF solicitation (bypasses SAM.gov)")
	flag.Parse()

	return Config{
		Mode:    *mode,
		APIKey:  os.Getenv("SAM_API_KEY"),
		Limit:   *limit,
		PDFFile: *pdfFile,
	}
}

// validateConfig validates configuration for SAM.gov modes.
func validateConfig(config *Config) error {
	switch config.Mode {
	case "cached", "live":
		// valid
	default:
		return fmt.Errorf("mode must be 'cached' or 'live', got: %s", config.Mode)
	}

	if config.Mode == "live" && config.APIKey == "" {
		return fmt.Errorf("SAM_API_KEY environment variable is required for live mode")
	}

	return nil
}

// getEnv returns the value of an environment variable or a default.
func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
