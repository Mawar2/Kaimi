// Package main is the entry point for outline-probe, a developer tool for manually
// exercising the Outline agent against real SAM.gov data.
//
// outline-probe is a diagnostic tool and is not part of the production pipeline.
// It is not covered by unit tests.
//
// Three modes:
//
//	cached  — use pre-recorded fixture data (default, no API key needed)
//	live    — search SAM.gov with real HTTP requests (requires SAM_API_KEY)
//
// The --pdf-file flag bypasses SAM.gov and runs the agent against a local PDF
// solicitation file (requires pdftotext from poppler-utils).
//
// Usage:
//
//	# Cached mode — no API key needed
//	go run ./cmd/outline-probe
//
//	# Live mode — searches SAM.gov, downloads PDFs, runs outline agent
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
	"path/filepath"
	"strings"
	"time"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/outline"
	"github.com/Mawar2/Kaimi/internal/profile"
	"github.com/Mawar2/Kaimi/internal/samgov"
)

// config holds the probe configuration.
type config struct {
	mode    string // "cached" or "live"
	limit   int    // max opportunities to process (live mode)
	apiKey  string // SAM.gov API key (live mode only)
	pdfFile string // local PDF path (overrides mode when set)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "outline-probe error: %v\n", err)
		os.Exit(1)
	}
}

// run contains the main probe logic.
func run() error {
	cfg := parseConfig()
	ctx := context.Background()

	if cfg.pdfFile != "" {
		return runPDFMode(ctx, cfg.pdfFile)
	}
	return runSAMMode(ctx, cfg)
}

// parseConfig reads configuration from command-line flags and environment variables.
func parseConfig() config {
	mode := flag.String("mode", getEnv("MODE", "cached"), "Mode: cached or live")
	limit := flag.Int("limit", 10, "Maximum number of opportunities to process (live mode)")
	pdfFile := flag.String("pdf-file", "", "Path to a local PDF solicitation (bypasses SAM.gov)")
	flag.Parse()

	return config{
		mode:    *mode,
		limit:   *limit,
		apiKey:  os.Getenv("SAM_API_KEY"),
		pdfFile: *pdfFile,
	}
}

// runSAMMode fetches opportunities from SAM.gov (cached or live) and runs the outline
// agent against each one.
func runSAMMode(ctx context.Context, cfg config) error {
	if cfg.mode != "cached" && cfg.mode != "live" {
		return fmt.Errorf("mode must be 'cached' or 'live', got: %s", cfg.mode)
	}
	if cfg.mode == "live" && cfg.apiKey == "" {
		return fmt.Errorf("SAM_API_KEY is required for live mode")
	}

	fmt.Printf("outline-probe starting (mode: %s)\n", cfg.mode)
	fmt.Printf("NAICS codes: %v\n\n", profile.BlueMeta.NAICSCodes)

	samClient, err := samgov.NewClient(samgov.Config{
		APIKey:    cfg.apiKey,
		UseCached: cfg.mode == "cached",
	})
	if err != nil {
		return fmt.Errorf("failed to create SAM.gov client: %w", err)
	}

	opps, err := samClient.FetchByNAICS(ctx, profile.BlueMeta.NAICSCodes)
	if err != nil {
		return fmt.Errorf("failed to fetch opportunities: %w", err)
	}

	fmt.Printf("Fetched %d opportunities\n", len(opps))

	// Apply limit in live mode to avoid burning SAM.gov quota.
	if cfg.mode == "live" && cfg.limit > 0 && len(opps) > cfg.limit {
		opps = opps[:cfg.limit]
		fmt.Printf("(limited to %d per --limit flag)\n", cfg.limit)
	}

	fmt.Println()

	a := outline.New()
	for i, opp := range opps {
		if err := probeOpportunity(ctx, a, opp, i+1, len(opps)); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to process %s: %v\n", opp.ID, err)
		}
	}

	fmt.Printf("outline-probe complete. Processed %d opportunities.\n", len(opps))
	return nil
}

// runPDFMode extracts text from a local PDF file and runs the outline agent on the
// resulting synthetic opportunity. Requires pdftotext (poppler-utils).
func runPDFMode(ctx context.Context, pdfPath string) error {
	abs, err := filepath.Abs(pdfPath)
	if err != nil {
		return fmt.Errorf("failed to resolve PDF path: %w", err)
	}

	fmt.Printf("outline-probe starting (mode: --pdf-file)\n")
	fmt.Printf("PDF: %s\n\n", abs)

	text, err := extractPDFText(abs)
	if err != nil {
		return fmt.Errorf("failed to extract PDF text: %w", err)
	}

	// Build a synthetic opportunity from the PDF text so the outline agent can
	// derive sections and formatting rules exactly as it would from a real opportunity.
	opp := &opportunity.Opportunity{
		ID:          "pdf-probe",
		Title:       strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath)),
		Description: text,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	a := outline.New()
	return probeOpportunity(ctx, a, opp, 1, 1)
}

// extractPDFText calls pdftotext to extract plain text from a PDF file.
// Requires pdftotext from the poppler-utils package.
func extractPDFText(path string) (string, error) {
	// "-layout" preserves whitespace layout; "-" writes to stdout.
	out, err := exec.Command("pdftotext", "-layout", path, "-").Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext failed (is poppler-utils installed?): %w", err)
	}
	return string(out), nil
}

// probeOpportunity runs the outline agent on one opportunity and prints formatted results.
func probeOpportunity(ctx context.Context, a *outline.Agent, opp *opportunity.Opportunity, idx, total int) error {
	fmt.Printf("═══════════════════════════════════════════════════════════\n")
	fmt.Printf("[%d/%d] %s\n", idx, total, opp.Title)
	if opp.ID != "" {
		fmt.Printf("        ID: %s", opp.ID)
		if opp.NAICSCode != "" {
			fmt.Printf(" | NAICS: %s", opp.NAICSCode)
		}
		if opp.SetAsideCode != "" {
			fmt.Printf(" | Set-Aside: %q", opp.SetAsideCode)
		}
		fmt.Println()
	}
	fmt.Println()

	ol, result, err := a.Run(ctx, opp)
	if err != nil {
		fmt.Printf("  [FAILED] %v\n\n", err)
		return err
	}

	fmt.Printf("  Status:  %s\n", result.Status)
	fmt.Printf("  Summary: %s\n\n", result.Summary)

	printSections(ol)
	printFormattingRules(ol.FormattingRules)
	fmt.Println()

	return nil
}

// printSections prints the derived proposal sections.
func printSections(ol *outline.Outline) {
	fmt.Printf("  Sections (%d):\n", len(ol.Sections))
	for _, s := range ol.Sections {
		fmt.Printf("    %-40s %s\n", s.Title, s.Rationale)
	}
}

// printFormattingRules prints the formatting rules extracted from the solicitation.
func printFormattingRules(rules *outline.FormattingRules) {
	fmt.Println()
	fmt.Println("  Formatting Rules:")
	printRule("Page Limit", rules.PageLimit)
	printRule("Font", rules.Font)
	printRule("Margins", rules.Margins)
	printRule("Line Spacing", rules.LineSpacing)
	printRule("File Format", rules.FileFormat)

	if len(rules.RequiredForms) > 0 {
		fmt.Printf("    %-16s %s\n", "Required Forms:", strings.Join(rules.RequiredForms, ", "))
	} else {
		fmt.Printf("    %-16s (not specified in solicitation)\n", "Required Forms:")
	}
}

// printRule prints one FormattingRule with its specified/unspecified state.
func printRule(name string, rule *outline.FormattingRule) {
	label := name + ":"
	if rule.Specified {
		fmt.Printf("    %-16s %s\n", label, rule.Value)
	} else {
		fmt.Printf("    %-16s (not specified in solicitation)\n", label)
	}
}

// getEnv returns the environment variable value or a default when the variable is unset.
func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
