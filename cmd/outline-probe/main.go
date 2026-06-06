// Package main implements outline-probe, a developer diagnostic tool for manually
// exercising the Outline agent against real or cached SAM.gov data.
//
// This tool is not part of the production pipeline and has no unit tests per issue #78.
// It is intended for developers to validate Outline agent behavior end-to-end.
//
// Three operating modes:
//
//	# Cached mode — reads test/fixtures/samgov_response.json (default, no API key needed)
//	go run ./cmd/outline-probe
//
//	# Live mode — fetches from real SAM.gov API, limited to N opportunities
//	SAM_API_KEY=your-key go run ./cmd/outline-probe --mode=live --limit=3
//
//	# PDF mode — extracts text from a local solicitation PDF via pdftotext
//	go run ./cmd/outline-probe --pdf-file=path/to/solicitation.pdf
//
// In all modes the tool prints sections derived and formatting rules extracted for
// each opportunity, with no eligibility filtering (diagnostic tool, not a gate).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/outline"
	"github.com/Mawar2/Kaimi/internal/profile"
	"github.com/Mawar2/Kaimi/internal/samgov"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "outline-probe: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	mode := flag.String("mode", "cached", "Mode: cached or live")
	limit := flag.Int("limit", 0, "Maximum opportunities to process in live mode (0 = unlimited)")
	pdfFile := flag.String("pdf-file", "", "Path to a local PDF solicitation; bypasses SAM.gov")
	flag.Parse()

	ctx := context.Background()

	if *pdfFile != "" {
		return runPDFMode(ctx, *pdfFile)
	}

	if *mode != "cached" && *mode != "live" {
		return fmt.Errorf("--mode must be 'cached' or 'live', got %q", *mode)
	}

	apiKey := os.Getenv("SAM_API_KEY")
	if *mode == "live" && apiKey == "" {
		return fmt.Errorf("SAM_API_KEY environment variable is required for --mode=live")
	}

	samClient, err := samgov.NewClient(samgov.Config{
		APIKey:    apiKey,
		UseCached: *mode == "cached",
	})
	if err != nil {
		return fmt.Errorf("creating SAM.gov client: %w", err)
	}

	fmt.Printf("outline-probe: mode=%s\n", *mode)
	fmt.Printf("NAICS codes: %s\n\n", strings.Join(profile.BlueMeta.NAICSCodes, ", "))

	opportunities, err := samClient.FetchByNAICS(ctx, profile.BlueMeta.NAICSCodes)
	if err != nil {
		return fmt.Errorf("fetching opportunities: %w", err)
	}

	fmt.Printf("Fetched %d opportunities (no eligibility filtering applied)\n\n", len(opportunities))

	if *limit > 0 && len(opportunities) > *limit {
		fmt.Printf("Limiting to first %d opportunities (--limit=%d)\n\n", *limit, *limit)
		opportunities = opportunities[:*limit]
	}

	return processOpportunities(ctx, opportunities)
}

// runPDFMode extracts solicitation text from a local PDF via pdftotext, then runs
// the Outline agent on a synthetic opportunity backed by the extracted text.
// Requires poppler-utils to be installed.
func runPDFMode(ctx context.Context, pdfPath string) error {
	fmt.Printf("outline-probe: mode=pdf-file path=%s\n\n", pdfPath)

	if _, err := os.Stat(pdfPath); err != nil {
		return fmt.Errorf("reading PDF: %w", err)
	}

	// pdftotext writes extracted text to stdout when the output argument is "-".
	out, err := exec.Command("pdftotext", pdfPath, "-").Output()
	if err != nil {
		return fmt.Errorf("pdftotext failed (is poppler-utils installed?): %w", err)
	}

	text := strings.TrimSpace(string(out))
	if text == "" {
		return fmt.Errorf("pdftotext produced no text from %s", pdfPath)
	}

	opp := &opportunity.Opportunity{
		ID:          "pdf-probe",
		Title:       pdfPath,
		Description: text,
	}

	return processOpportunities(ctx, []*opportunity.Opportunity{opp})
}

// processOpportunities runs the Outline agent on each opportunity and prints results.
func processOpportunities(ctx context.Context, opportunities []*opportunity.Opportunity) error {
	ag := outline.New()
	errorCount := 0

	for _, opp := range opportunities {
		fmt.Printf("=== %s ===\n", opp.Title)
		fmt.Printf("  ID:        %s\n", opp.ID)
		fmt.Printf("  Set-aside: %q\n", opp.SetAsideCode)

		ol, result, err := ag.Run(ctx, opp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ERROR: %v\n\n", err)
			errorCount++
			continue
		}

		fmt.Printf("  Status:  %s\n", result.Status)
		fmt.Printf("  Summary: %s\n", result.Summary)

		fmt.Printf("  Sections (%d):\n", len(ol.Sections))
		for _, s := range ol.Sections {
			req := "required"
			if !s.Required {
				req = "optional"
			}
			fmt.Printf("    [%s] %s\n", req, s.Title)
			fmt.Printf("      rationale: %s\n", s.Rationale)
		}

		fmt.Printf("  Formatting rules:\n")
		fr := ol.FormattingRules
		printRule("Page limit  ", fr.PageLimit)
		printRule("Font        ", fr.Font)
		printRule("Margins     ", fr.Margins)
		printRule("Line spacing", fr.LineSpacing)
		printRule("File format ", fr.FileFormat)
		if len(fr.RequiredForms) > 0 {
			fmt.Printf("    Required forms: %s\n", strings.Join(fr.RequiredForms, ", "))
		} else {
			fmt.Printf("    Required forms: (none specified)\n")
		}
		fmt.Println()
	}

	fmt.Printf("outline-probe complete: %d opportunities processed, %d errors\n",
		len(opportunities), errorCount)
	if errorCount > 0 {
		return fmt.Errorf("%d opportunities failed outline generation", errorCount)
	}
	return nil
}

// printRule prints one FormattingRule in a consistent diagnostic format.
func printRule(label string, rule *outline.FormattingRule) {
	if rule.Specified {
		fmt.Printf("    %s: %s\n", label, rule.Value)
	} else {
		fmt.Printf("    %s: (not specified)\n", label)
	}
}
