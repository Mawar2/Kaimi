// Package main implements seed-demo, a dev probe that seeds a demo store with
// real, active opportunities from SAM.gov's nightly Contract Opportunities CSV
// extract (ContractOpportunitiesFullCSV.csv, published quota-free on SAM.gov
// Data Services). See issue #272.
//
// The CSV carries full description text (the live v2 search API returns only a
// description URL — see issue #268), so seeded opportunities give the Scorer
// and Writer real solicitation language to ground on. Rows are filtered to
// active notices matching the capability profile's NAICS codes, posted
// recently, with a response deadline still in the future. Seeded opportunities
// are unscored so cmd/scorer picks them up.
//
// Example usage:
//
//	go run ./cmd/seed-demo --csv=ContractOpportunitiesFullCSV.csv
//	go run ./cmd/seed-demo --csv=... --store-path=hackathon/demo-store --max-per-naics=5 --days=30
package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/profile"
	"github.com/Mawar2/Kaimi/internal/store"
)

// minDescriptionChars is the smallest description we consider real solicitation
// text. Shorter values are usually placeholders ("See attachment."), which
// defeat the purpose of seeding from the CSV.
const minDescriptionChars = 200

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "seed-demo error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	csvPath := flag.String("csv", "", "Path to ContractOpportunitiesFullCSV.csv (required)")
	storePath := flag.String("store-path", "hackathon/demo-store", "Store directory to seed")
	profilePath := flag.String("profile", "config/bluemeta_profile.yaml", "Capability profile path")
	days := flag.Int("days", 30, "Only seed opportunities posted within this many days")
	maxPerNAICS := flag.Int("max-per-naics", 5, "Cap on seeded opportunities per NAICS code (newest first)")
	flag.Parse()

	if *csvPath == "" {
		return fmt.Errorf("--csv is required")
	}

	prof, err := profile.LoadProfile(*profilePath)
	if err != nil {
		return fmt.Errorf("failed to load capability profile: %w", err)
	}
	codes := make(map[string]bool)
	naicsDesc := make(map[string]string)
	for _, nc := range prof.NAICSCodes {
		codes[nc.Code] = true
		naicsDesc[nc.Code] = nc.Description
	}

	f, err := os.Open(*csvPath)
	if err != nil {
		return fmt.Errorf("failed to open CSV: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only file

	now := time.Now()
	cutoff := now.AddDate(0, 0, -*days)

	candidates, scanned, err := collectCandidates(f, codes, naicsDesc, cutoff, now)
	if err != nil {
		return err
	}

	seeded, err := saveTopPerNAICS(context.Background(), *storePath, candidates, *maxPerNAICS)
	if err != nil {
		return err
	}

	fmt.Printf("Scanned %d CSV rows, matched %d candidates, seeded %d opportunities into %s\n",
		scanned, len(candidates), seeded, *storePath)
	return nil
}

// collectCandidates streams the CSV and returns every row that passes the
// seeding filters, mapped to the canonical Opportunity schema.
func collectCandidates(r io.Reader, codes map[string]bool, naicsDesc map[string]string, cutoff, now time.Time) ([]*opportunity.Opportunity, int, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // tolerate ragged rows in the government extract

	header, err := reader.Read()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read CSV header: %w", err)
	}
	cols := indexColumns(header)
	for _, required := range []string{"NoticeId", "Title", "PostedDate", "ResponseDeadLine", "NaicsCode", "Active", "Description"} {
		if _, ok := cols[required]; !ok {
			return nil, 0, fmt.Errorf("CSV is missing required column %q", required)
		}
	}

	var candidates []*opportunity.Opportunity
	scanned := 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Skip malformed rows rather than failing the whole seed run.
			fmt.Fprintf(os.Stderr, "Warning: skipping malformed CSV row: %v\n", err)
			continue
		}
		scanned++

		if !shouldSeed(cols, row, codes, cutoff, now) {
			continue
		}
		opp, err := rowToOpportunity(cols, row, naicsDesc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping row %s: %v\n", field(cols, row, "NoticeId"), err)
			continue
		}
		candidates = append(candidates, opp)
	}
	return candidates, scanned, nil
}

// saveTopPerNAICS keeps the newest maxPerNAICS candidates for each NAICS code
// and persists them through the Store interface.
func saveTopPerNAICS(ctx context.Context, storePath string, candidates []*opportunity.Opportunity, maxPerNAICS int) (int, error) {
	st, err := store.NewJSONStore(storePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open store at %s: %w", storePath, err)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].PostedDate.After(candidates[j].PostedDate)
	})

	perNAICS := make(map[string]int)
	seeded := 0
	for _, opp := range candidates {
		if perNAICS[opp.NAICSCode] >= maxPerNAICS {
			continue
		}
		if err := st.Save(ctx, opp); err != nil {
			return seeded, fmt.Errorf("failed to save opportunity %s: %w", opp.ID, err)
		}
		perNAICS[opp.NAICSCode]++
		seeded++
	}
	return seeded, nil
}

// indexColumns maps CSV header names to their column positions.
func indexColumns(header []string) map[string]int {
	cols := make(map[string]int, len(header))
	for i, name := range header {
		cols[strings.TrimSpace(name)] = i
	}
	return cols
}

// field returns the named column from a row, or "" when the column is absent
// or the row is too short (the extract has occasional ragged rows).
func field(cols map[string]int, row []string, name string) string {
	i, ok := cols[name]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// shouldSeed reports whether a CSV row is demo-worthy: an active notice in one
// of the profile's NAICS codes, posted after cutoff, with a response deadline
// still in the future and real description text.
func shouldSeed(cols map[string]int, row []string, codes map[string]bool, cutoff, now time.Time) bool {
	if !strings.EqualFold(field(cols, row, "Active"), "yes") {
		return false
	}
	if !codes[field(cols, row, "NaicsCode")] {
		return false
	}
	posted, err := parseCSVTime(field(cols, row, "PostedDate"))
	if err != nil || posted.Before(cutoff) {
		return false
	}
	deadline, err := parseCSVTime(field(cols, row, "ResponseDeadLine"))
	if err != nil || !deadline.After(now) {
		return false
	}
	return len(field(cols, row, "Description")) >= minDescriptionChars
}

// rowToOpportunity maps a CSV row to the canonical Opportunity schema,
// populating only the Hunter-owned fields so the Scorer treats it as unscored.
func rowToOpportunity(cols map[string]int, row []string, naicsDesc map[string]string) (*opportunity.Opportunity, error) {
	posted, err := parseCSVTime(field(cols, row, "PostedDate"))
	if err != nil {
		return nil, fmt.Errorf("bad PostedDate: %w", err)
	}
	deadline, err := parseCSVTime(field(cols, row, "ResponseDeadLine"))
	if err != nil {
		return nil, fmt.Errorf("bad ResponseDeadLine: %w", err)
	}

	naics := field(cols, row, "NaicsCode")
	now := time.Now().UTC()
	return &opportunity.Opportunity{
		ID:                 field(cols, row, "NoticeId"),
		Title:              field(cols, row, "Title"),
		SolicitationNum:    field(cols, row, "Sol#"),
		Agency:             field(cols, row, "Department/Ind.Agency"),
		Office:             field(cols, row, "Office"),
		PostedDate:         posted,
		ResponseDeadline:   deadline,
		NAICSCode:          naics,
		NAICSDescription:   naicsDesc[naics],
		SetAsideCode:       field(cols, row, "SetASideCode"),
		PlaceOfPerformance: formatPlace(cols, row),
		Description:        field(cols, row, "Description"),
		Type:               field(cols, row, "Type"),
		URL:                field(cols, row, "Link"),
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}

// parseCSVTime parses the timestamp formats that appear in the extract,
// e.g. "2026-06-10 23:21:50.45-04", RFC3339 deadlines, and bare dates.
func parseCSVTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	formats := []string{
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05-07",
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp %q", s)
}

// formatPlace builds a human-readable place of performance from the Pop* columns.
func formatPlace(cols map[string]int, row []string) string {
	var parts []string
	for _, name := range []string{"PopCity", "PopState", "PopZip"} {
		if v := field(cols, row, name); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, ", ")
}
