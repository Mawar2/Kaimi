package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Mawar2/Kaimi/internal/proposal"
)

// TestDraftEditorPage verifies the standalone full-page draft editor: it loads
// the selected proposal's document, renders the section rail + editable doc, and
// is NOT wrapped in the app shell (no sidebar).
func TestDraftEditorPage(t *testing.T) {
	h, svc, _ := newProposalHandler(t)
	if rr := postForm(t, h, "/opportunity/zta-1/select", url.Values{}); rr.Code != http.StatusSeeOther {
		t.Fatalf("select: status %d, want 303", rr.Code)
	}
	svc.Wait()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/editor/zta-1", http.NoBody))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /editor/zta-1: status %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		`class="ed-fullpage`, // the focused full-page surface
		`class="ed-rail"`,    // section rail
		"Back to review",     // returns to the workspace
		"<textarea",          // editable sections
		`data-autosave`,      // reuses the workspace autosave
		"Executive Summary",  // a real document section
	} {
		if !contains(body, want) {
			t.Errorf("/editor missing %q", want)
		}
	}
	// Standalone page: no app-shell sidebar.
	if contains(body, `class="side"`) {
		t.Errorf("editor must be a standalone page (no app shell sidebar)")
	}
}

// TestEditorRequiresDocument 404s a proposal that was never selected.
func TestEditorRequiresDocument(t *testing.T) {
	h, _, _ := newProposalHandler(t)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/editor/zta-1", http.NoBody))
	if rr.Code != http.StatusNotFound {
		t.Errorf("editor for an unselected opp: status %d, want 404", rr.Code)
	}
}

// gapBody is a section body holding two unresolved Writer gap markers plus a
// script tag, so the same fixture proves the aggregated gap bar (one bar, not
// one callout per gap — issue #274) and the escaping.
const gapBody = "Staffed by [GAP: number of cleared staff] engineers holding [GAP: facility clearance level] clearances. <script>alert(1)</script>"

// seedGapSection selects zta-1 and writes gapBody into its first section.
func seedGapSection(t *testing.T, h http.Handler, svc *proposal.Service) string {
	t.Helper()
	if rr := postForm(t, h, "/opportunity/zta-1/select", url.Values{}); rr.Code != http.StatusSeeOther {
		t.Fatalf("select: status %d, want 303", rr.Code)
	}
	svc.Wait()
	doc, err := svc.Document("zta-1")
	if err != nil {
		t.Fatalf("Document: %v", err)
	}
	secID := doc.Sections[0].ID
	if rr := postForm(t, h, "/workspace/zta-1/section/"+secID, url.Values{"body": {gapBody}}); rr.Code != http.StatusSeeOther {
		t.Fatalf("section save: status %d, want 303", rr.Code)
	}
	return secID
}

// TestEditorAggregatesUnresolvedGaps: a section holding two [GAP: ...]
// markers gets the amber textarea tint, ONE aggregated gap bar (count +
// next-gap cycling + expandable list — issue #274, not one callout per gap),
// and a warn mark in the section rail — and the gap text is HTML-escaped.
func TestEditorAggregatesUnresolvedGaps(t *testing.T) {
	h, svc, _ := newProposalHandler(t)
	seedGapSection(t, h, svc)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/editor/zta-1", http.NoBody))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /editor/zta-1: status %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		`class="gap-warn"`,         // amber textarea tint
		"2 unresolved gaps",        // aggregated count, not per-gap callouts
		"data-gapnext",             // cycle-through-gaps control
		"data-gaptoggle",           // expandable gap list
		"number of cleared staff",  // first missing fact, in the list
		"facility clearance level", // second missing fact, in the list
		`class="ed-sec warn"`,      // section rail warn mark
		"function gapTexts",        // client-side live recount script
	} {
		if !contains(body, want) {
			t.Errorf("/editor missing %q", want)
		}
	}
	if got := strings.Count(body, "data-gapbar>"); got != 1 {
		t.Errorf("a 2-gap section must render exactly 1 visible gap bar, got %d", got)
	}
	if contains(body, "Find in text") {
		t.Errorf("per-gap 'Find in text' buttons must be replaced by the aggregated bar")
	}
	if contains(body, "<script>alert(1)</script>") {
		t.Errorf("section body with markup must be HTML-escaped")
	}
}

// TestEditorNoGaps_NoWarnUI: a clean draft renders without visible gap UI —
// the bar is present but hidden, so the live recount can reveal it if the
// human types a new [GAP: ...] marker.
func TestEditorNoGaps_NoWarnUI(t *testing.T) {
	h, svc, _ := newProposalHandler(t)
	if rr := postForm(t, h, "/opportunity/zta-1/select", url.Values{}); rr.Code != http.StatusSeeOther {
		t.Fatalf("select: status %d, want 303", rr.Code)
	}
	svc.Wait()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/editor/zta-1", http.NoBody))
	body := rr.Body.String()
	for _, reject := range []string{`class="gap-warn"`, `class="ed-sec warn"`} {
		if contains(body, reject) {
			t.Errorf("clean draft must not render %q", reject)
		}
	}
	if contains(body, "data-gapbar") && !contains(body, "data-gapbar hidden") {
		t.Errorf("clean draft must render gap bars hidden")
	}
	// The design system sets display:flex on .ed-flag, which beats the UA's
	// [hidden] default — the stylesheet must re-assert it or hidden bars show
	// as "0 unresolved gaps" callouts.
	if !contains(body, ".ed-gap[hidden]") {
		t.Errorf("stylesheet must keep [hidden] gap bars display:none")
	}
}

// TestWorkspaceGateAggregatesGaps: the review-gate section editors get the
// same aggregated gap bar as the full editor, plus a top-of-page summary
// ("N unresolved gaps across M sections") with anchor links to the sections.
func TestWorkspaceGateAggregatesGaps(t *testing.T) {
	h, svc, _ := newProposalHandler(t)
	seedGapSection(t, h, svc)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/workspace/zta-1", http.NoBody))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /workspace/zta-1: status %d", rr.Code)
	}
	body := rr.Body.String()
	// The gate keeps the aggregated summary + the entry point to the full editor,
	// but the per-section gap bars and next-gap controls now live ONLY on the full
	// editor (the gate stays a clean review surface).
	for _, want := range []string{
		`class="gap-warn"`,                   // a textarea with gaps is still highlighted
		"data-gapsummary",                    // top-of-page summary block
		"2 unresolved gaps across 1 section", // summary headline
		`href="#gsec-`,                       // summary anchor link to the section
		"Open full editor mode",              // entry point to gap resolution
	} {
		if !contains(body, want) {
			t.Errorf("/workspace gate missing %q", want)
		}
	}
	// Match the rendered elements (trailing ">"), not the shared JS selectors
	// ("[data-gapbar]") that the live-recount script still references.
	if got := strings.Count(body, "data-gapbar>"); got != 0 {
		t.Errorf("the gate must not render per-section gap bars (they live on the full editor now), got %d", got)
	}
	if contains(body, "data-gapnext>") {
		t.Errorf("the gate must not render the next-gap control")
	}

	// The full editor still carries the per-section gap bar + next-gap control.
	er := httptest.NewRecorder()
	h.ServeHTTP(er, httptest.NewRequest("GET", "/editor/zta-1", http.NoBody))
	if er.Code != http.StatusOK {
		t.Fatalf("GET /editor/zta-1: status %d", er.Code)
	}
	ebody := er.Body.String()
	for _, want := range []string{"data-gapbar>", "data-gapnext>", "unresolved gap"} {
		if !contains(ebody, want) {
			t.Errorf("/editor missing %q (gap controls must live here)", want)
		}
	}
}

// TestWorkspaceGateNoGaps_SummaryHidden: a clean draft at the gate renders the
// summary hidden so the live recount can reveal it if a gap is introduced.
func TestWorkspaceGateNoGaps_SummaryHidden(t *testing.T) {
	h, svc, _ := newProposalHandler(t)
	if rr := postForm(t, h, "/opportunity/zta-1/select", url.Values{}); rr.Code != http.StatusSeeOther {
		t.Fatalf("select: status %d, want 303", rr.Code)
	}
	svc.Wait()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/workspace/zta-1", http.NoBody))
	body := rr.Body.String()
	if contains(body, "data-gapsummary") && !contains(body, "data-gapsummary hidden") {
		t.Errorf("clean draft must render the gap summary hidden")
	}
	if contains(body, `class="gap-warn"`) {
		t.Errorf("clean draft must not tint any textarea")
	}
}
