// Package main is the entry point for the outline-probe developer diagnostic tool.
//
// outline-probe exercises the Outline agent against real SAM.gov data. It is a
// developer tool only — not part of the production pipeline — and does not apply
// the eligibility gate (every fetched opportunity is processed so the developer
// can see how the agent behaves on all set-aside types).
//
// Three modes:
//   - cached (default): reads from test/fixtures/samgov_response.json, no API key needed
//   - live: real SAM.gov API, requires SAM_API_KEY, --limit caps the run
//   - --pdf-file: extract text from a local PDF via pdftotext, bypasses SAM.gov entirely
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
	"path/filepath"
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

// run contains the main logic for the outline-probe diagnostic tool.
func run() error {
	mode := flag.String("mode", "cached", "Mode: cached or live")
	limit := flag.Int("limit", 10, "Max opportunities to process (live mode only)")
	pdfFilePath := flag.String("pdf-file", "", "Local PDF to process via pdftotext (bypasses SAM.gov)")
	flag.Parse()

	ctx := context.Background()

	if *pdfFilePath != "" {
		return runPDFMode(ctx, *pdfFilePath)
	}
	return runSAMMode(ctx, *mode, *limit)
}

// runSAMMode fetches opportunities from SAM.gov (cached or live) and processes them.
func runSAMMode(ctx context.Context, mode string, limit int) error {
	if mode != "cached" && mode != "live" {
		return fmt.Errorf("mode must be 'cached' or 'live', got: %s", mode)
	}

	apiKey := os.Getenv("SAM_API_KEY")
	if mode == "live" && apiKey == "" {
		return fmt.Errorf("SAM_API_KEY environment variable is required for live mode")
	}

	client, err := samgov.NewClient(samgov.Config{
		APIKey:    apiKey,
		UseCached: mode == "cached",
	})
	if err != nil {
		return fmt.Errorf("failed to create SAM.gov client: %w", err)
	}

	fmt.Printf("outline-probe — mode: %s\n", mode)
	fmt.Printf("NAICS codes: %v\n\n", profile.BlueMeta.NAICSCodes)

	opps, err := client.FetchByNAICS(ctx, profile.BlueMeta.NAICSCodes)
	if err != nil {
		return fmt.Errorf("failed to fetch opportunities: %w", err)
	}

	// Diagnostic tool: no eligibility filter. Process everything so the developer
	// can observe Outline behaviour on all set-aside types.
	if mode == "live" && len(opps) > limit {
		opps = opps[:limit]
	}

	fmt.Printf("Fetched %d opportunities (eligibility gate disabled — diagnostic tool)\n\n", len(opps))

	return processOpportunities(ctx, opps)
}

// runPDFMode extracts text from a local PDF file using pdftotext and runs the
// Outline agent against that text. Useful for testing against specific solicitations
// without a SAM.gov API call.
func runPDFMode(ctx context.Context, pdfFilePath string) error {
	// pdftotext is part of poppler-utils; write "-" as the output path for stdout.
	cmd := exec.Command("pdftotext", pdfFilePath, "-")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("pdftotext failed (is poppler-utils installed?): %w", err)
	}

	text := string(out)
	base := filepath.Base(pdfFilePath)
	name := strings.TrimSuffix(base, filepath.Ext(base))

	fmt.Printf("outline-probe — mode: pdf-file\n")
	fmt.Printf("File: %s (%d chars extracted)\n\n", pdfFilePath, len(text))

	opp := &opportunity.Opportunity{
		ID:          "pdf:" + name,
		Title:       name,
		Description: text,
	}

	return processOpportunities(ctx, []*opportunity.Opportunity{opp})
}

// processOpportunities runs the Outline agent against each opportunity and prints
// the derived sections and formatting rules.
func processOpportunities(ctx context.Context, opps []*opportunity.Opportunity) error {
	ag := outline.New()

	for i, opp := range opps {
		printDivider()
		fmt.Printf("[%d/%d] %s\n", i+1, len(opps), opp.Title)
		fmt.Printf("  ID:        %s\n", opp.ID)

		if opp.SetAsideCode != "" {
			fmt.Printf("  Set-aside: %s\n", opp.SetAsideCode)
		} else {
			fmt.Printf("  Set-aside: (none — full and open)\n")
		}
		fmt.Println()

		ol, result, err := ag.Run(ctx, opp)
		if err != nil {
			fmt.Printf("  ERROR: %v\n\n", err)
			continue
		}
		if result.IsFailed() {
			fmt.Printf("  FAILED: %s\n\n", result.Summary)
			continue
		}

		printSections(ol.Sections)
		printFormattingRules(ol.FormattingRules)
	}

	printDivider()
	fmt.Printf("Done. %d opportunit%s processed.\n", len(opps), pluralSuffix(len(opps)))
	return nil
}

// printSections writes the derived section list to stdout.
func printSections(sections []outline.Section) {
	fmt.Printf("  Sections (%d):\n", len(sections))
	for _, s := range sections {
		fmt.Printf("    • %-42s [%s]\n", s.Title, s.Rationale)
	}
	fmt.Println()
}

// printFormattingRules writes the extracted formatting requirements to stdout.
func printFormattingRules(rules *outline.FormattingRules) {
	fmt.Println("  Formatting rules:")
	printRule("Page limit", rules.PageLimit)
	printRule("Font", rules.Font)
	printRule("Margins", rules.Margins)
	printRule("Line spacing", rules.LineSpacing)
	printRule("File format", rules.FileFormat)

	if len(rules.RequiredForms) > 0 {
		fmt.Printf("    %-16s %s\n", "Required forms:", strings.Join(rules.RequiredForms, ", "))
	} else {
		fmt.Printf("    %-16s (none specified)\n", "Required forms:")
	}
	fmt.Println()
}

// printRule prints a single formatting rule, noting when it is unspecified.
func printRule(label string, rule *outline.FormattingRule) {
	key := label + ":"
	if rule.Specified {
		fmt.Printf("    %-16s %s\n", key, rule.Value)
	} else {
		fmt.Printf("    %-16s (not specified)\n", key)
	}
}

// printDivider prints a visual separator between opportunity blocks.
func printDivider() {
	fmt.Println("─────────────────────────────────────────────────────")
}

// pluralSuffix returns "y" for count==1 and "ies" otherwise, for the word "opportunit".
func pluralSuffix(count int) string {
	if count == 1 {
		return "y"
	}
	return "ies"
}
