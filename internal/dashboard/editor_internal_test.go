package dashboard

import (
	"strings"
	"testing"

	"github.com/Mawar2/Kaimi/internal/document"
)

// TestHighlightGaps verifies the read-only draft rendering: gap markers are
// wrapped in <mark>, surrounding prose is preserved, and the body is
// HTML-escaped before the wrapper is added (no injection path).
func TestHighlightGaps(t *testing.T) {
	got := string(highlightGaps("Staffed by [GAP: cleared staff count] engineers."))
	want := `Staffed by <mark class="gap-mark">[GAP: cleared staff count]</mark> engineers.`
	if got != want {
		t.Errorf("highlightGaps:\n got %q\nwant %q", got, want)
	}
}

func TestHighlightGaps_EscapesMarkup(t *testing.T) {
	got := string(highlightGaps(`<script>x</script> [GAP: a <b> tag]`))
	if strings.Contains(got, "<script>") || strings.Contains(got, "<b>") {
		t.Errorf("body markup must be escaped, got %q", got)
	}
	if !strings.Contains(got, `<mark class="gap-mark">`) {
		t.Errorf("gap marker must still be wrapped, got %q", got)
	}
}

func TestHighlightGaps_NoMarker_PlainEscape(t *testing.T) {
	got := string(highlightGaps("No gaps here & none expected."))
	if strings.Contains(got, "<mark") {
		t.Errorf("no marker, no mark: %q", got)
	}
	if !strings.Contains(got, "&amp;") {
		t.Errorf("plain text must still be escaped: %q", got)
	}
}

// TestSummarizeGaps verifies the review-gate summary aggregation (issue
// #274): totals across sections, only gap-holding sections listed, and a
// correctly pluralized headline.
func TestSummarizeGaps(t *testing.T) {
	sections := []document.Section{
		{ID: "exec", Heading: "Executive Summary", Body: "All grounded prose."},
		{ID: "tech", Heading: "Technical Approach", Body: "Staffed by [GAP: cleared staff count] holding [GAP: clearance level]."},
		{ID: "past", Heading: "Past Performance", Body: "Delivered before. [GAP: DoD contract number]"},
	}
	got := summarizeGaps(sections)
	if got.Total != 3 {
		t.Errorf("Total = %d, want 3", got.Total)
	}
	if len(got.Sections) != 2 {
		t.Fatalf("Sections = %d entries, want 2 (only gap-holding sections)", len(got.Sections))
	}
	if got.Sections[0].ID != "tech" || got.Sections[0].Count != 2 {
		t.Errorf("Sections[0] = %+v, want tech with 2 gaps", got.Sections[0])
	}
	if got.Headline != "3 unresolved gaps across 2 sections" {
		t.Errorf("Headline = %q", got.Headline)
	}
}

func TestSummarizeGaps_Singular(t *testing.T) {
	got := summarizeGaps([]document.Section{
		{ID: "tech", Heading: "Technical Approach", Body: "[GAP: staffing count]"},
	})
	if got.Headline != "1 unresolved gap across 1 section" {
		t.Errorf("Headline = %q", got.Headline)
	}
}

func TestSummarizeGaps_Clean(t *testing.T) {
	got := summarizeGaps([]document.Section{{ID: "exec", Heading: "Exec", Body: "fine"}})
	if got.Total != 0 || len(got.Sections) != 0 {
		t.Errorf("clean draft: got %+v, want zero summary", got)
	}
}

// TestOpenNonGapFlags: the gate's flag banners must exclude resolved flags and
// the per-gap "Unresolved gap" flags — gaps are surfaced by the aggregated
// summary instead (issue #274), so persisted gap flags would double-report.
func TestOpenNonGapFlags(t *testing.T) {
	flags := []document.Flag{
		{Title: gapFlagTitle, Detail: "missing staffing count", SectionID: "tech"},
		{Title: "Tone concern", Detail: "too informal", SectionID: "exec"},
		{Title: "Stale citation", Detail: "old contract", SectionID: "past", Resolved: true},
	}
	got := openNonGapFlags(flags)
	if len(got) != 1 || got[0].Title != "Tone concern" {
		t.Errorf("openNonGapFlags = %+v, want only the open non-gap flag", got)
	}
}
