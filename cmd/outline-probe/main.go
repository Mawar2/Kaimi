// Package main is the entry point for the outline-probe developer diagnostic tool.
//
// outline-probe manually exercises the Outline agent against real SAM.gov data.
// It is NOT part of the production pipeline and has no unit tests.
//
// Three modes are supported:
//
//   - cached (default): reads test/fixtures/samgov_response.json; no API key required
//   - live: fetches from the real SAM.gov API; requires SAM_API_KEY; --limit caps results
//   - --pdf-file: extracts text from a local PDF via pdftotext; bypasses SAM.gov entirely
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

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "outline-probe error: %v\n", err)
		os.Exit(1)
	}
}

// run is the main entry point, separated for testability.
func run() error {
	mode := flag.String("mode", getEnv("MODE", "cached"), "Mode: cached or live")
	limit := flag.Int("limit", 0, "Maximum opportunities to process in live mode (0 = unlimited)")
	pdfFile := flag.String("pdf-file", "", "Path to a local PDF file to probe (bypasses SAM.gov)")
	flag.Parse()

	ctx := context.Background()

	// --pdf-file takes priority over --mode.
	if *pdfFile != "" {
		return probePDF(ctx, *pdfFile)
	}

	return probeByMode(ctx, *mode, *limit)
}

// probeByMode fetches opportunities via SAM.gov (cached or live) and runs the Outline agent.
func probeByMode(ctx context.Context, mode string, limit int) error {
	if mode != "cached" && mode != "live" {
		return fmt.Errorf("mode must be 'cached' or 'live', got: %s", mode)
	}

	apiKey := os.Getenv("SAM_API_KEY")
	if mode == "live" && apiKey == "" {
		return fmt.Errorf("SAM_API_KEY environment variable is required for live mode")
	}

	samClient, err := samgov.NewClient(samgov.Config{
		APIKey:    apiKey,
		UseCached: mode == "cached",
	})
	if err != nil {
		return fmt.Errorf("failed to create SAM.gov client: %w", err)
	}

	fmt.Printf("outline-probe — mode: %s\n", mode)
	fmt.Printf("NAICS codes: %s\n", strings.Join(profile.BlueMeta.NAICSCodes, ", "))
	fmt.Println(strings.Repeat("─", 60))

	opps, err := samClient.FetchByNAICS(ctx, profile.BlueMeta.NAICSCodes)
	if err != nil {
		return fmt.Errorf("failed to fetch opportunities: %w", err)
	}

	// No eligibility filtering: diagnostic tool processes every opportunity.
	if limit > 0 && len(opps) > limit {
		opps = opps[:limit]
	}

	fmt.Printf("Fetched %d opportunities — processing all (no eligibility gate)\n\n", len(opps))
	return processOpportunities(ctx, opps)
}

// probePDF extracts text from a local PDF via pdftotext and runs the Outline agent.
func probePDF(ctx context.Context, path string) error {
	fmt.Printf("outline-probe — mode: pdf-file\n")
	fmt.Printf("File: %s\n", path)
	fmt.Println(strings.Repeat("─", 60))

	text, err := extractPDFText(path)
	if err != nil {
		return fmt.Errorf("failed to extract PDF text: %w", err)
	}

	now := time.Now().UTC()
	opp := &opportunity.Opportunity{
		ID:          "pdf-file",
		Title:       fmt.Sprintf("PDF: %s", path),
		Description: text,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	return processOpportunities(ctx, []*opportunity.Opportunity{opp})
}

// extractPDFText runs pdftotext -layout on path and returns the extracted text.
// pdftotext is part of poppler-utils; an informative error is returned if absent.
func extractPDFText(path string) (string, error) {
	// -layout preserves column layout; "-" writes output to stdout.
	cmd := exec.Command("pdftotext", "-layout", path, "-")
	out, err := cmd.Output()
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") ||
			strings.Contains(err.Error(), "not found") {
			return "", fmt.Errorf("pdftotext not found: install poppler-utils (e.g. apt install poppler-utils)")
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("pdftotext failed (exit %d): %s", ee.ExitCode(), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("pdftotext failed: %w", err)
	}
	return string(out), nil
}

// processOpportunities runs the Outline agent on each opportunity and prints results.
func processOpportunities(ctx context.Context, opps []*opportunity.Opportunity) error {
	agent := outline.New()
	errors := 0

	for i, opp := range opps {
		fmt.Printf("[%d/%d] %s\n", i+1, len(opps), opp.Title)
		fmt.Printf("       ID: %s\n", opp.ID)

		ol, result, err := agent.Run(ctx, opp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ERROR: %v\n\n", err)
			errors++
			continue
		}

		if result != nil && result.IsFailed() {
			fmt.Fprintf(os.Stderr, "  FAILED: %s\n\n", result.Summary)
			errors++
			continue
		}

		printOutline(ol)
		fmt.Println()
	}

	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Done. %d/%d opportunities processed successfully.\n", len(opps)-errors, len(opps))
	if errors > 0 {
		return fmt.Errorf("%d opportunities failed", errors)
	}
	return nil
}

// printOutline prints sections and formatting rules for a single Outline.
func printOutline(ol *outline.Outline) {
	fmt.Printf("       Sections (%d):\n", len(ol.Sections))
	for _, s := range ol.Sections {
		marker := "✓"
		if !s.Required {
			marker = "○"
		}
		fmt.Printf("         %s %s — %s\n", marker, s.Title, s.Rationale)
	}

	fmt.Println("       Formatting rules:")
	fr := ol.FormattingRules
	printRule("         Page limit", fr.PageLimit)
	printRule("         Font", fr.Font)
	printRule("         Margins", fr.Margins)
	printRule("         Line spacing", fr.LineSpacing)
	printRule("         File format", fr.FileFormat)

	if len(fr.RequiredForms) > 0 {
		fmt.Printf("         Required forms: %s\n", strings.Join(fr.RequiredForms, ", "))
	} else {
		fmt.Println("         Required forms: (not specified)")
	}
}

// printRule prints a single FormattingRule with a label.
func printRule(label string, rule *outline.FormattingRule) {
	if rule == nil || !rule.Specified {
		fmt.Printf("%s: (not specified)\n", label)
		return
	}
	fmt.Printf("%s: %s\n", label, rule.Value)
}

// getEnv returns the value of an environment variable or a default value.
func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
