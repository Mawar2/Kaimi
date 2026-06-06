// Package main is the entry point for the outline-probe developer diagnostic tool.
//
// outline-probe manually exercises the Outline agent against real or fixture SAM.gov data.
// It is a developer diagnostic tool and not part of the production pipeline.
//
// Three modes:
//   - cached (default): reads test/fixtures/samgov_response.json; no API key required
//   - live:             queries real SAM.gov API; requires SAM_API_KEY; use --limit to cap results
//   - --pdf-file:       extracts text from a local PDF via pdftotext; bypasses SAM.gov entirely
//
// All fetched opportunities are processed without eligibility filtering — this is a
// diagnostic tool, not a gate.
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

func run() error {
	mode := flag.String("mode", "cached", "Mode: cached or live")
	limit := flag.Int("limit", 5, "Max opportunities to process in live mode")
	pdfFile := flag.String("pdf-file", "", "Path to a local PDF solicitation; bypasses SAM.gov entirely")
	flag.Parse()

	ctx := context.Background()

	var (
		opps []*opportunity.Opportunity
		err  error
	)

	switch {
	case *pdfFile != "":
		opps, err = opportunitiesFromPDF(*pdfFile)
	case *mode == "live":
		apiKey := os.Getenv("SAM_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("SAM_API_KEY environment variable is required for live mode")
		}
		opps, err = fetchFromSAMGov(ctx, apiKey, false, *limit)
	case *mode == "cached":
		opps, err = fetchFromSAMGov(ctx, "", true, 0)
	default:
		return fmt.Errorf("mode must be 'cached' or 'live', got: %s", *mode)
	}
	if err != nil {
		return err
	}

	fmt.Printf("outline-probe: processing %d opportunities\n\n", len(opps))

	ag := outline.New()
	for i, opp := range opps {
		fmt.Printf("=== Opportunity %d/%d: %s ===\n", i+1, len(opps), opp.Title)
		fmt.Printf("ID:        %s\n", opp.ID)
		fmt.Printf("Set-aside: %s\n", setAsideLabel(opp.SetAsideCode))

		ol, result, runErr := ag.Run(ctx, opp)
		if runErr != nil {
			fmt.Printf("ERROR: %v\n\n", runErr)
			continue
		}

		fmt.Printf("Status:    %s\n", result.Status)
		fmt.Printf("Summary:   %s\n", result.Summary)

		fmt.Printf("\nSections derived (%d):\n", len(ol.Sections))
		for j, s := range ol.Sections {
			req := "optional"
			if s.Required {
				req = "required"
			}
			fmt.Printf("  %d. [%s] %s\n", j+1, req, s.Title)
			fmt.Printf("     Rationale: %s\n", s.Rationale)
		}

		fmt.Printf("\nFormatting rules extracted:\n")
		printRule("  Page limit  ", ol.FormattingRules.PageLimit)
		printRule("  Font        ", ol.FormattingRules.Font)
		printRule("  Margins     ", ol.FormattingRules.Margins)
		printRule("  Line spacing", ol.FormattingRules.LineSpacing)
		printRule("  File format ", ol.FormattingRules.FileFormat)
		if len(ol.FormattingRules.RequiredForms) > 0 {
			fmt.Printf("  Required forms: %s\n", strings.Join(ol.FormattingRules.RequiredForms, ", "))
		} else {
			fmt.Printf("  Required forms: (not specified)\n")
		}
		fmt.Println()
	}

	return nil
}

// fetchFromSAMGov fetches opportunities from SAM.gov using the cached or live client.
// When limit > 0, the result is truncated to that count (live mode only).
// No eligibility filter is applied — this is a diagnostic tool.
func fetchFromSAMGov(ctx context.Context, apiKey string, useCached bool, limit int) ([]*opportunity.Opportunity, error) {
	client, err := samgov.NewClient(samgov.Config{
		APIKey:    apiKey,
		UseCached: useCached,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create SAM.gov client: %w", err)
	}

	opps, err := client.FetchByNAICS(ctx, profile.BlueMeta.NAICSCodes)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from SAM.gov: %w", err)
	}

	if limit > 0 && len(opps) > limit {
		opps = opps[:limit]
	}

	return opps, nil
}

// opportunitiesFromPDF creates a synthetic opportunity from a local PDF using pdftotext.
// pdftotext must be installed (poppler-utils package).
func opportunitiesFromPDF(path string) ([]*opportunity.Opportunity, error) {
	// "-" as the output argument writes extracted text to stdout.
	out, err := exec.Command("pdftotext", path, "-").Output() //nolint:gosec // path is from CLI flag, not user input
	if err != nil {
		return nil, fmt.Errorf("pdftotext failed for %q (is poppler-utils installed?): %w", path, err)
	}

	text := strings.TrimSpace(string(out))
	if text == "" {
		return nil, fmt.Errorf("pdftotext returned empty text for %q", path)
	}

	now := time.Now().UTC()
	opp := &opportunity.Opportunity{
		ID:          "pdf-probe",
		Title:       fmt.Sprintf("PDF: %s", path),
		Description: text,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	return []*opportunity.Opportunity{opp}, nil
}

// setAsideLabel returns a human-readable label for a SAM.gov set-aside code.
func setAsideLabel(code string) string {
	if code == "" {
		return "(full and open)"
	}
	return code
}

// printRule prints a single formatting rule showing its value or "(not specified)".
func printRule(label string, rule *outline.FormattingRule) {
	if rule.Specified {
		fmt.Printf("%s: %s\n", label, rule.Value)
	} else {
		fmt.Printf("%s: (not specified)\n", label)
	}
}
