package main

import (
	"testing"
	"time"
)

// sampleHeader mirrors the column names in ContractOpportunitiesFullCSV.csv
// (only the columns rowToOpportunity reads, at arbitrary positions).
var sampleHeader = []string{
	"NoticeId", "Title", "Sol#", "Department/Ind.Agency", "Office",
	"PostedDate", "Type", "SetASideCode", "SetASide", "ResponseDeadLine",
	"NaicsCode", "PopCity", "PopState", "PopZip", "Active", "Link", "Description",
}

func sampleRow() []string {
	return []string{
		"abc123def456",
		"Cloud Migration Support Services",
		"W91QVP26R0001",
		"DEPT OF DEFENSE",
		"AQ HQ CONTRACTING",
		"2026-06-10 23:21:50.45-04",
		"Solicitation",
		"SBA",
		"Total Small Business Set-Aside (FAR 19.5)",
		"2026-07-15T13:00:00-04:00",
		"541512",
		"Honolulu",
		"HI",
		"96858",
		"Yes",
		"https://sam.gov/opp/abc123def456/view",
		"The Government requires cloud migration and systems integration support for enterprise workloads, " +
			"including assessment of the current on-premises environment, design of the target cloud architecture, " +
			"migration execution, and post-migration operations and maintenance support across the period of performance.",
	}
}

func TestRowToOpportunity(t *testing.T) {
	cols := indexColumns(sampleHeader)
	naicsDesc := map[string]string{"541512": "Computer Systems Design Services"}

	opp, err := rowToOpportunity(cols, sampleRow(), naicsDesc)
	if err != nil {
		t.Fatalf("rowToOpportunity returned error: %v", err)
	}

	if opp.ID != "abc123def456" {
		t.Errorf("ID = %q, want %q", opp.ID, "abc123def456")
	}
	if opp.Title != "Cloud Migration Support Services" {
		t.Errorf("Title = %q", opp.Title)
	}
	if opp.SolicitationNum != "W91QVP26R0001" {
		t.Errorf("SolicitationNum = %q", opp.SolicitationNum)
	}
	if opp.Agency != "DEPT OF DEFENSE" {
		t.Errorf("Agency = %q", opp.Agency)
	}
	if opp.NAICSCode != "541512" {
		t.Errorf("NAICSCode = %q", opp.NAICSCode)
	}
	if opp.NAICSDescription != "Computer Systems Design Services" {
		t.Errorf("NAICSDescription = %q", opp.NAICSDescription)
	}
	if opp.SetAsideCode != "SBA" {
		t.Errorf("SetAsideCode = %q", opp.SetAsideCode)
	}
	if opp.URL != "https://sam.gov/opp/abc123def456/view" {
		t.Errorf("URL = %q", opp.URL)
	}
	if opp.PlaceOfPerformance != "Honolulu, HI, 96858" {
		t.Errorf("PlaceOfPerformance = %q", opp.PlaceOfPerformance)
	}
	if opp.Description == "" || opp.Description[:14] != "The Government" {
		t.Errorf("Description not mapped as text: %q", opp.Description)
	}

	wantPosted := time.Date(2026, 6, 10, 23, 21, 50, 450000000, time.FixedZone("", -4*3600))
	if !opp.PostedDate.Equal(wantPosted) {
		t.Errorf("PostedDate = %v, want %v", opp.PostedDate, wantPosted)
	}
	wantDeadline := time.Date(2026, 7, 15, 13, 0, 0, 0, time.FixedZone("", -4*3600))
	if !opp.ResponseDeadline.Equal(wantDeadline) {
		t.Errorf("ResponseDeadline = %v, want %v", opp.ResponseDeadline, wantDeadline)
	}

	// Seeded opportunities must be unscored so the real Scorer picks them up.
	if opp.Score != 0 || opp.ScoredAt != nil {
		t.Errorf("seeded opportunity must be unscored, got score=%v scoredAt=%v", opp.Score, opp.ScoredAt)
	}
	if opp.Selected {
		t.Error("seeded opportunity must not be pre-selected")
	}
}

func TestShouldSeed(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	cutoff := now.AddDate(0, 0, -30)
	codes := map[string]bool{"541512": true}

	cols := indexColumns(sampleHeader)
	base := sampleRow()

	cases := []struct {
		name   string
		mutate func(r []string)
		want   bool
	}{
		{"valid row", func(r []string) {}, true},
		{"inactive", func(r []string) { r[14] = "No" }, false},
		{"wrong NAICS", func(r []string) { r[10] = "336413" }, false},
		{"posted too old", func(r []string) { r[5] = "2026-01-02 10:00:00.00-04" }, false},
		{"deadline in the past", func(r []string) { r[9] = "2026-06-01T13:00:00-04:00" }, false},
		{"no description text", func(r []string) { r[16] = "" }, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := append([]string(nil), base...)
			tc.mutate(row)
			got := shouldSeed(cols, row, codes, cutoff, now)
			if got != tc.want {
				t.Errorf("shouldSeed = %v, want %v", got, tc.want)
			}
		})
	}
}
