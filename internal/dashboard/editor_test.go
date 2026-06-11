package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
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
