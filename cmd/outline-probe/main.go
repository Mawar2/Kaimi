// outline-probe fetches opportunities matching BlueMeta's capability profile,
// downloads attached solicitation PDFs, extracts their text with pdftotext, and
// runs the Outline agent to show what formatting rules and sections are produced.
//
// Usage:
//
//	# Cached mode (no API key needed — uses test/fixtures/samgov_response.json)
//	go run ./cmd/outline-probe
//
//	# Live mode: searches SAM.gov using BlueMeta's NAICS codes
//	SAM_API_KEY=your-key go run ./cmd/outline-probe --mode=live --limit=3
//
//	# Local PDF: extract text from a file on disk and run the outline agent
//	go run ./cmd/outline-probe --pdf-file=path/to/solicitation.pdf
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/outline"
	"github.com/Mawar2/Kaimi/internal/profile"
	"github.com/Mawar2/Kaimi/internal/samgov"
)

// htmlTagRE strips HTML tags from SAM.gov description pointer responses.
var htmlTagRE = regexp.MustCompile(`<[^>]+>`)

// pdfToText is the path to the Poppler pdftotext binary.
const pdfToText = `C:\Program Files\Git\mingw64\bin\pdftotext.exe`

func main() {
	mode := flag.String("mode", "cached", "Mode: cached or live")
	limit := flag.Int("limit", 3, "Max number of opportunities to process")
	pdfFile := flag.String("pdf-file", "", "Path to a PDF file to parse directly (bypasses SAM.gov fetch)")
	flag.Parse()

	if err := run(*mode, *limit, *pdfFile); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(mode string, limit int, pdfFile string) error {
	apiKey := os.Getenv("SAM_API_KEY")
	if mode == "live" && apiKey == "" {
		return fmt.Errorf("SAM_API_KEY environment variable required for live mode")
	}

	prof := profile.BlueMeta

	fmt.Printf("Capability profile: BlueMeta Technologies\n")
	fmt.Printf("NAICS codes: %s\n", strings.Join(prof.NAICSCodes, ", "))
	fmt.Println(strings.Repeat("=", 60))

	// --pdf-file mode: extract text from a local PDF and run the outline agent directly.
	if pdfFile != "" {
		return runPDFFile(pdfFile)
	}

	client, err := samgov.NewClient(samgov.Config{
		APIKey:    apiKey,
		UseCached: mode == "cached",
	})
	if err != nil {
		return fmt.Errorf("create SAM.gov client: %w", err)
	}

	ctx := context.Background()
	opps, err := client.FetchByNAICS(ctx, prof.NAICSCodes)
	if err != nil {
		return fmt.Errorf("fetch opportunities: %w", err)
	}

	if len(opps) == 0 {
		fmt.Println("No opportunities found.")
		return nil
	}

	if len(opps) > limit {
		opps = opps[:limit]
	}

	fmt.Printf("Processing %d opportunity/ies (mode=%s)\n\n", len(opps), mode)

	a := outline.New()
	tmpDir, err := os.MkdirTemp("", "outline-probe-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	for _, opp := range opps {
		processOpportunity(ctx, a, opp, apiKey, mode, tmpDir)
	}

	return nil
}

// runPDFFile extracts text from a local PDF and runs the outline agent on it.
func runPDFFile(pdfPath string) error {
	fmt.Printf("PDF file: %s\n\n", pdfPath)

	tmpDir, err := os.MkdirTemp("", "outline-probe-pdf-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	text, err := extractPDFText(pdfPath, tmpDir)
	if err != nil {
		return err
	}
	fmt.Printf("Extracted %d chars of PDF text.\n\n", len(text))

	opp := &opportunity.Opportunity{
		ID:          "PDF-PROBE",
		Title:       filepath.Base(pdfPath),
		Agency:      "(from local PDF)",
		Description: text,
	}

	ctx := context.Background()
	ol, result, err := outline.New().Run(ctx, opp)
	if err != nil {
		return fmt.Errorf("outline agent: %w", err)
	}
	printOutline(opp, ol, string(result.Status))
	return nil
}

// processOpportunity resolves the best available description text for an opportunity
// (PDF attachment → inline description → pointer URL), runs the outline agent, and prints results.
func processOpportunity(ctx context.Context, a *outline.Agent, opp *opportunity.Opportunity, apiKey, mode, tmpDir string) {
	fmt.Printf("Opportunity: %s\n", opp.Title)
	fmt.Printf("ID:          %s\n", opp.ID)

	// Priority 1: download the first PDF attachment and extract its text.
	if mode == "live" && len(opp.Attachments) > 0 {
		for i, attURL := range opp.Attachments {
			fmt.Printf("Fetching attachment %d/%d...\n", i+1, len(opp.Attachments))
			text, err := downloadAndExtractPDF(attURL, tmpDir)
			if err != nil {
				fmt.Printf("  Warning: %v\n", err)
				continue
			}
			fmt.Printf("  Extracted %d chars of PDF text.\n", len(text))
			opp.Description = text
			break
		}
	}

	// Priority 2: follow a SAM.gov description pointer URL.
	if mode == "live" && strings.HasPrefix(opp.Description, "https://api.sam.gov") {
		fmt.Printf("Fetching full description from pointer URL...\n")
		fullDesc, err := fetchDescription(opp.Description, apiKey)
		if err != nil {
			fmt.Printf("  Warning: could not fetch description: %v\n", err)
		} else {
			opp.Description = fullDesc
			fmt.Printf("  Got %d chars of description text.\n", len(fullDesc))
		}
	}

	ol, result, err := a.Run(ctx, opp)
	if err != nil {
		fmt.Printf("[FAIL] %v\n\n", err)
		return
	}
	printOutline(opp, ol, string(result.Status))
}

// downloadAndExtractPDF downloads a URL, saves it as a PDF, and returns extracted plain text.
func downloadAndExtractPDF(url, tmpDir string) (string, error) {
	resp, err := http.Get(url) //nolint:noctx // probe-only CLI tool; no request cancellation needed
	if err != nil {
		return "", fmt.Errorf("HTTP get: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close response body: %v\n", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "pdf") && !strings.HasSuffix(strings.ToLower(url), ".pdf") {
		return "", fmt.Errorf("not a PDF (Content-Type: %s)", ct)
	}

	pdfPath := filepath.Join(tmpDir, "attachment.pdf")
	f, err := os.Create(pdfPath)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	if _, err = io.Copy(f, resp.Body); err != nil {
		_ = f.Close() // best-effort close on write error path
		return "", fmt.Errorf("write PDF: %w", err)
	}
	if err = f.Close(); err != nil {
		return "", fmt.Errorf("close PDF file: %w", err)
	}

	return extractPDFText(pdfPath, tmpDir)
}

// extractPDFText runs pdftotext on pdfPath and returns the resulting plain text.
func extractPDFText(pdfPath, tmpDir string) (string, error) {
	txtPath := filepath.Join(tmpDir, "extracted.txt")
	cmd := exec.Command(pdfToText, pdfPath, txtPath) //nolint:gosec // G204: pdfToText is a compile-time constant, not user input
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("pdftotext: %w — %s", err, strings.TrimSpace(string(out)))
	}
	raw, err := os.ReadFile(txtPath)
	if err != nil {
		return "", fmt.Errorf("read extracted text: %w", err)
	}
	return strings.Join(strings.Fields(string(raw)), " "), nil
}

// fetchDescription follows a SAM.gov description pointer URL and returns plain text.
func fetchDescription(url, apiKey string) (string, error) {
	if !strings.Contains(url, "api_key=") {
		sep := "&"
		if !strings.Contains(url, "?") {
			sep = "?"
		}
		url = url + sep + "api_key=" + apiKey
	}

	resp, err := http.Get(url) //nolint:noctx // probe-only CLI tool; no request cancellation needed
	if err != nil {
		return "", fmt.Errorf("HTTP get: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close response body: %v\n", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("SAM.gov returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	// SAM.gov wraps the description in {"description":"<html>..."}.
	text := string(body)
	text = strings.TrimPrefix(text, `{"description":"`)
	text = strings.TrimSuffix(text, `"}`)
	text = strings.ReplaceAll(text, `\n`, " ")
	text = strings.ReplaceAll(text, `\r`, "")
	text = strings.ReplaceAll(text, `\"`, `"`)
	text = htmlTagRE.ReplaceAllString(text, " ")
	return strings.Join(strings.Fields(text), " "), nil
}

func printOutline(opp *opportunity.Opportunity, ol *outline.Outline, status string) {
	fmt.Printf("Agency:      %s\n", opp.Agency)
	fmt.Printf("Status:      %s\n", status)
	fmt.Printf("Sections:    %d\n", len(ol.Sections))
	for _, s := range ol.Sections {
		fmt.Printf("  - [%s] %s\n", s.ID, s.Title)
		fmt.Printf("      Rationale: %s\n", s.Rationale)
	}

	fmt.Println("\nFormatting Rules:")
	fr := ol.FormattingRules
	printRule("  Page limit  ", fr.PageLimit)
	printRule("  Font        ", fr.Font)
	printRule("  Margins     ", fr.Margins)
	printRule("  Line spacing", fr.LineSpacing)
	printRule("  File format ", fr.FileFormat)
	if len(fr.RequiredForms) > 0 {
		fmt.Printf("  Forms       : %s\n", strings.Join(fr.RequiredForms, ", "))
	} else {
		fmt.Println("  Forms       : not specified")
	}

	fmt.Println("\nDescription excerpt (first 500 chars):")
	desc := opp.Description
	if len(desc) > 500 {
		desc = desc[:500] + "..."
	}
	fmt.Printf("  %s\n", desc)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println()
}

func printRule(label string, r *outline.FormattingRule) {
	if r.Specified {
		fmt.Printf("%s: %s\n", label, r.Value)
	} else {
		fmt.Printf("%s: not specified\n", label)
	}
}
