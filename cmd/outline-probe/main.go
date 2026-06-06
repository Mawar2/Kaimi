// Package main is the entry point for the outline-probe developer tool.
//
// outline-probe exercises the Outline agent end-to-end against real or fixture data.
// It is a developer tool — not part of the production pipeline — and is not covered
// by unit tests.
//
// Three modes:
//
//	cached (default):  reads from test/fixtures/samgov_response.json; no API key needed
//	live:              fetches real opportunities from SAM.gov; enriches descriptions from PDFs
//	--pdf-file <path>: extracts text from a local PDF and runs the Outline agent on it
//
// Usage:
//
//	# Cached mode — no API key needed
//	go run ./cmd/outline-probe
//
//	# Live mode — searches SAM.gov, downloads PDFs, runs outline agent
//	SAM_API_KEY=your-key go run ./cmd/outline-probe --mode=live --limit=3
//
//	# Local PDF — extract text from a file on disk, bypass SAM.gov entirely
//	go run ./cmd/outline-probe --pdf-file=path/to/solicitation.pdf
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/outline"
	"github.com/Mawar2/Kaimi/internal/profile"
	"github.com/Mawar2/Kaimi/internal/samgov"
)

// Config holds the outline-probe configuration.
type Config struct {
	Mode    string // "cached" or "live"
	APIKey  string // SAM.gov API key (live mode only)
	Limit   int    // Max opportunities to process in live mode (0 = no limit)
	PDFFile string // Local PDF path; when set, --mode and --limit are ignored
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "outline-probe error: %v\n", err)
		os.Exit(1)
	}
}

// run is the main entry point, extracted so errors can be returned naturally.
func run() error {
	config := parseConfig()
	if err := validateConfig(&config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	ctx := context.Background()
	a := outline.New()

	// --pdf-file takes precedence over --mode.
	if config.PDFFile != "" {
		return runPDFFileMode(ctx, a, config.PDFFile)
	}

	return runSAMGovMode(ctx, a, config)
}

// parseConfig reads configuration from environment variables and command-line flags.
// API keys are read from the environment only — never from flags — to avoid leaking
// them into shell history.
func parseConfig() Config {
	mode := flag.String("mode", getEnv("MODE", "cached"), "Mode: cached or live")
	limit := flag.Int("limit", 0, "Max opportunities to process in live mode (0 = no limit)")
	pdfFile := flag.String("pdf-file", "", "Path to a local PDF; skips SAM.gov entirely")

	flag.Parse()

	return Config{
		Mode:    *mode,
		APIKey:  os.Getenv("SAM_API_KEY"),
		Limit:   *limit,
		PDFFile: *pdfFile,
	}
}

// validateConfig returns an error if the configuration is invalid.
func validateConfig(config *Config) error {
	// --pdf-file mode bypasses mode/API-key requirements.
	if config.PDFFile != "" {
		if _, err := os.Stat(config.PDFFile); err != nil {
			return fmt.Errorf("--pdf-file: %w", err)
		}
		return nil
	}

	if config.Mode != "cached" && config.Mode != "live" {
		return fmt.Errorf("--mode must be 'cached' or 'live', got: %q", config.Mode)
	}
	if config.Mode == "live" && config.APIKey == "" {
		return fmt.Errorf("SAM_API_KEY environment variable is required for live mode")
	}
	return nil
}

// runPDFFileMode extracts text from a local PDF and runs the Outline agent on it,
// bypassing SAM.gov entirely.
func runPDFFileMode(ctx context.Context, a *outline.Agent, path string) error {
	fmt.Printf("outline-probe — pdf-file mode\n")
	fmt.Printf("PDF: %s\n\n", path)

	text, err := extractPDFText(path)
	if err != nil {
		return fmt.Errorf("failed to extract PDF text: %w", err)
	}
	fmt.Printf("Extracted %d chars from PDF\n\n", len(text))

	opp := &opportunity.Opportunity{
		ID:          "pdf-probe",
		Title:       path,
		Description: text,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	return printOutline(ctx, a, opp)
}

// runSAMGovMode fetches opportunities from SAM.gov (cached or live) and runs the
// Outline agent on each, printing the results.
func runSAMGovMode(ctx context.Context, a *outline.Agent, config Config) error {
	fmt.Printf("outline-probe — %s mode\n", config.Mode)
	fmt.Printf("NAICS codes: %v\n\n", profile.BlueMeta.NAICSCodes)

	samClient, err := samgov.NewClient(samgov.Config{
		APIKey:    config.APIKey,
		UseCached: config.Mode == "cached",
	})
	if err != nil {
		return fmt.Errorf("failed to create SAM.gov client: %w", err)
	}

	opportunities, err := samClient.FetchByNAICS(ctx, profile.BlueMeta.NAICSCodes)
	if err != nil {
		return fmt.Errorf("failed to fetch opportunities: %w", err)
	}
	fmt.Printf("Found %d opportunities\n\n", len(opportunities))

	if len(opportunities) == 0 {
		fmt.Println("No opportunities found — nothing to probe.")
		return nil
	}

	// Apply limit in live mode.
	if config.Limit > 0 && len(opportunities) > config.Limit {
		opportunities = opportunities[:config.Limit]
		fmt.Printf("Processing first %d opportunities (--limit)\n\n", config.Limit)
	}

	for i, opp := range opportunities {
		fmt.Printf("=== Opportunity %d/%d ===\n", i+1, len(opportunities))
		fmt.Printf("ID:    %s\n", opp.ID)
		fmt.Printf("Title: %s\n", opp.Title)

		// In live mode, try to enrich the description from PDF attachments.
		if config.Mode == "live" {
			enrichFromAttachments(ctx, opp)
		}

		fmt.Printf("Description: %d chars\n\n", len(opp.Description))

		if err := printOutline(ctx, a, opp); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: outline agent failed for %s: %v\n", opp.ID, err)
		}
		fmt.Println()
	}

	return nil
}

// enrichFromAttachments downloads PDF attachments and appends extracted text to the
// opportunity description. Failures are logged as warnings and do not abort processing.
func enrichFromAttachments(ctx context.Context, opp *opportunity.Opportunity) {
	if len(opp.Attachments) == 0 {
		return
	}
	fmt.Printf("  Enriching from %d attachment(s)...\n", len(opp.Attachments))

	for _, url := range opp.Attachments {
		if !strings.HasSuffix(strings.ToLower(url), ".pdf") {
			continue
		}
		text, err := downloadAndExtract(ctx, url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: failed to extract %s: %v\n", url, err)
			continue
		}
		fmt.Printf("  Extracted %d chars from %s\n", len(text), url)
		opp.Description += "\n\n" + text
	}
}

// downloadAndExtract downloads a PDF from the given URL to a temp file, then runs
// pdftotext on it and returns the extracted text.
func downloadAndExtract(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close response body: %v\n", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	tmpFile, err := os.CreateTemp("", "outline-probe-*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		_ = tmpFile.Close() // Ignore close error; io.Copy error takes precedence.
		return "", fmt.Errorf("failed to write PDF: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	return extractPDFText(tmpFile.Name())
}

// extractPDFText runs pdftotext on the file at path and returns the extracted text.
// Returns a descriptive error if pdftotext is not installed.
func extractPDFText(path string) (string, error) {
	// pdftotext path - : "-" sends output to stdout.
	cmd := exec.Command("pdftotext", path, "-")
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("pdftotext failed (exit %d): %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("pdftotext not available: %w (install poppler-utils)", err)
	}
	return string(out), nil
}

// printOutline runs the Outline agent on opp and prints the resulting sections and
// formatting rules to stdout.
func printOutline(ctx context.Context, a *outline.Agent, opp *opportunity.Opportunity) error {
	ol, result, err := a.Run(ctx, opp)
	if err != nil {
		return fmt.Errorf("outline agent: %w", err)
	}

	fmt.Printf("Status:    %s\n", result.Status)
	fmt.Printf("Summary:   %s\n", result.Summary)
	fmt.Printf("Generated: %s\n\n", ol.GeneratedAt.Format(time.RFC3339))

	fmt.Printf("Sections (%d):\n", len(ol.Sections))
	for _, s := range ol.Sections {
		req := "optional"
		if s.Required {
			req = "required"
		}
		fmt.Printf("  [%-30s] %s (%s)\n", s.ID, s.Title, req)
		fmt.Printf("    rationale: %s\n", s.Rationale)
	}

	fmt.Println("\nFormatting Rules:")
	printFormattingRule("Page Limit", ol.FormattingRules.PageLimit)
	printFormattingRule("Font", ol.FormattingRules.Font)
	printFormattingRule("Margins", ol.FormattingRules.Margins)
	printFormattingRule("Line Spacing", ol.FormattingRules.LineSpacing)
	printFormattingRule("File Format", ol.FormattingRules.FileFormat)
	if len(ol.FormattingRules.RequiredForms) > 0 {
		fmt.Printf("  %-14s %s\n", "Required Forms:", strings.Join(ol.FormattingRules.RequiredForms, ", "))
	}

	return nil
}

// printFormattingRule prints a single formatting rule, distinguishing between
// explicitly stated rules and those absent from the solicitation.
func printFormattingRule(name string, rule *outline.FormattingRule) {
	if rule.Specified {
		fmt.Printf("  %-14s %s\n", name+":", rule.Value)
	} else {
		fmt.Printf("  %-14s (not specified)\n", name+":")
	}
}

// getEnv returns the value of the named environment variable, or defaultValue
// when the variable is unset or empty.
func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
