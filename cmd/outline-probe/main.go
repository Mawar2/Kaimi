// Package main is the outline-probe developer diagnostic tool.
//
// outline-probe exercises the Outline agent against SAM.gov opportunities.
// It is NOT part of the production pipeline; it exists solely for interactive
// inspection during development. No unit tests exist for this tool by design.
//
// Modes:
//   - cached (default): reads from test/fixtures/samgov_response.json; no API key needed
//   - live: queries the live SAM.gov API; requires SAM_API_KEY env var
//   - --pdf-file: extracts text from a local PDF via pdftotext and runs the agent
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
	"time"

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
	mode := flag.String("mode", getEnv("MODE", "cached"), "Mode: cached or live")
	limit := flag.Int("limit", 0, "Max opportunities to process in live mode (0 = no limit)")
	pdfFile := flag.String("pdf-file", "", "Path to a local PDF solicitation (processed via pdftotext)")
	flag.Parse()

	ctx := context.Background()
	agent := outline.New()

	// pdf-file mode: bypass SAM.gov entirely, extract text via pdftotext.
	if *pdfFile != "" {
		opp, err := opportunityFromPDF(*pdfFile)
		if err != nil {
			return fmt.Errorf("reading PDF: %w", err)
		}
		return processOpportunity(ctx, agent, opp)
	}

	// cached or live mode: validate mode flag.
	switch *mode {
	case "cached", "live":
		// valid
	default:
		return fmt.Errorf("mode must be 'cached' or 'live', got: %q", *mode)
	}

	naicsCodes := profile.BlueMeta.NAICSCodes
	fmt.Printf("outline-probe — mode: %s\n", *mode)
	fmt.Printf("NAICS codes: %s\n", strings.Join(naicsCodes, ", "))
	fmt.Println()

	useCached := *mode == "cached"
	apiKey := ""
	if !useCached {
		apiKey = os.Getenv("SAM_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("SAM_API_KEY environment variable is required for live mode")
		}
	}

	samClient, err := samgov.NewClient(samgov.Config{
		APIKey:    apiKey,
		UseCached: useCached,
	})
	if err != nil {
		return fmt.Errorf("creating SAM.gov client: %w", err)
	}

	fmt.Println("Fetching opportunities...")
	opportunities, err := samClient.FetchByNAICS(ctx, naicsCodes)
	if err != nil {
		return fmt.Errorf("fetching opportunities: %w", err)
	}

	// Apply limit (live mode only; cached has a fixed fixture set).
	if *limit > 0 && len(opportunities) > *limit {
		opportunities = opportunities[:*limit]
	}

	fmt.Printf("Processing %d opportunities (no eligibility filter — diagnostic mode)\n\n", len(opportunities))

	errorCount := 0
	for _, opp := range opportunities {
		if err := processOpportunity(ctx, agent, opp); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
			errorCount++
		}
		fmt.Println()
	}

	fmt.Println("--- outline-probe summary ---")
	fmt.Printf("opportunities: %d  errors: %d\n", len(opportunities), errorCount)
	return nil
}

// processOpportunity runs the Outline agent on one opportunity and prints the results.
func processOpportunity(ctx context.Context, a *outline.Agent, opp *opportunity.Opportunity) error {
	setAside := opp.SetAsideCode
	if setAside == "" {
		setAside = "(none)"
	}
	fmt.Printf("=== %s ===\n", opp.Title)
	fmt.Printf("    ID:        %s\n", opp.ID)
	fmt.Printf("    Agency:    %s\n", opp.Agency)
	fmt.Printf("    Set-aside: %s\n", setAside)
	fmt.Printf("    NAICS:     %s\n", opp.NAICSCode)

	o, result, err := a.Run(ctx, opp)
	if err != nil {
		return fmt.Errorf("outline agent failed for %s: %w", opp.ID, err)
	}
	fmt.Printf("    Status:    %s — %s\n\n", result.Status, result.Summary)

	fmt.Printf("  Sections (%d):\n", len(o.Sections))
	for i, s := range o.Sections {
		req := "required"
		if !s.Required {
			req = "optional"
		}
		fmt.Printf("    %2d. [%s] %s\n", i+1, req, s.Title)
		fmt.Printf("        → %s\n", s.Rationale)
	}

	fmt.Println()
	fmt.Println("  Formatting rules:")
	printRule("Page limit", o.FormattingRules.PageLimit)
	printRule("Font", o.FormattingRules.Font)
	printRule("Margins", o.FormattingRules.Margins)
	printRule("Line spacing", o.FormattingRules.LineSpacing)
	printRule("File format", o.FormattingRules.FileFormat)
	if len(o.FormattingRules.RequiredForms) > 0 {
		fmt.Printf("    Required forms:  %s\n", strings.Join(o.FormattingRules.RequiredForms, ", "))
	} else {
		fmt.Println("    Required forms:  (none specified)")
	}

	return nil
}

// printRule prints one formatting rule, noting when the solicitation was silent.
func printRule(name string, rule *outline.FormattingRule) {
	label := name + ":"
	if rule == nil || !rule.Specified {
		fmt.Printf("    %-15s (not specified)\n", label)
	} else {
		fmt.Printf("    %-15s %s\n", label, rule.Value)
	}
}

// opportunityFromPDF extracts solicitation text from a local PDF via pdftotext
// and wraps it in a synthetic Opportunity for diagnostic processing.
//
// Requires pdftotext (poppler-utils) on PATH.
func opportunityFromPDF(path string) (*opportunity.Opportunity, error) {
	// pdftotext <file> - writes extracted text to stdout.
	out, err := exec.Command("pdftotext", path, "-").Output()
	if err != nil {
		return nil, fmt.Errorf("pdftotext failed (is poppler-utils installed?): %w", err)
	}

	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	now := time.Now().UTC()

	return &opportunity.Opportunity{
		ID:          "pdf-probe",
		Title:       name,
		Agency:      "(local PDF)",
		Description: string(out),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// getEnv returns the value of an environment variable or a default.
func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
