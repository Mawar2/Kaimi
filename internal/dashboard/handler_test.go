package dashboard_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/dashboard"
	"github.com/Mawar2/Kaimi/internal/opportunity"
)

// newDetailHandler creates a Handler backed by a temp store seeded with one
// richly-populated opportunity suitable for detail page tests.
// It reuses newTestStore from view_test.go (same package dashboard_test).
func newDetailHandler(t *testing.T) (*dashboard.Handler, *opportunity.Opportunity) {
	t.Helper()
	s := newTestStore(t)

	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	scoredAt := now.Add(-24 * time.Hour)
	selectedAt := now.Add(-12 * time.Hour)

	opp := &opportunity.Opportunity{
		ID:                 "opp-detail-1",
		Title:              "Test Federal Contract",
		Agency:             "Department of Defense",
		Office:             "Army Corps",
		SolicitationNum:    "W52P1J-26-R-0001",
		NAICSCode:          "541511",
		NAICSDescription:   "Custom Computer Programming",
		SetAsideCode:       "SBA",
		PlaceOfPerformance: "Washington, DC",
		Type:               "Solicitation",
		ContractType:       "Firm Fixed Price",
		URL:                "https://sam.gov/opp/test",
		Description:        "This is the full opportunity description.",
		Score:              0.873,
		ScoreReasoning:     "Strong alignment with capability profile.",
		Recommendation:     "BID",
		Requirements:       []string{"Clearance required", "5 years experience"},
		ScoredAt:           &scoredAt,
		Selected:           true,
		SelectedAt:         &selectedAt,
		ProposalStatus:     "outline:success",
		PostedDate:         now.Add(-7 * 24 * time.Hour),
		ResponseDeadline:   now.Add(5 * 24 * time.Hour), // within 7 days → DeadlineSoon
		CreatedAt:          now.Add(-8 * 24 * time.Hour),
		UpdatedAt:          now,
	}

	if err := s.Save(context.Background(), opp); err != nil {
		t.Fatalf("newDetailHandler: Save: %v", err)
	}

	svc := dashboard.NewService(s)
	h := dashboard.NewHandler(svc)
	return h, opp
}

// TestHandleDetail_ValidID checks that a known ID returns 200.
func TestHandleDetail_ValidID(t *testing.T) {
	h, opp := newDetailHandler(t)

	req := httptest.NewRequest("GET", "/opportunity/"+opp.ID, http.NoBody)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body excerpt: %s", rr.Code, rr.Body.String()[:min(200, rr.Body.Len())])
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

// TestHandleDetail_NotFound checks that an unknown (but syntactically valid) ID returns 404
// with a human-readable message rather than a 500 error.
func TestHandleDetail_NotFound(t *testing.T) {
	h, _ := newDetailHandler(t)

	req := httptest.NewRequest("GET", "/opportunity/no-such-id", http.NoBody)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "not found") {
		t.Errorf("body should mention 'not found'; got: %s", body)
	}
}

// TestHandleDetail_InvalidID checks that IDs with injection/traversal characters return 404.
// '!' and '%2F' (encoded '/') are valid in URL paths but outside the ID allowlist [a-zA-Z0-9_-].
func TestHandleDetail_InvalidID(t *testing.T) {
	h, _ := newDetailHandler(t)

	// URL-safe characters that fail the [a-zA-Z0-9_-] allowlist.
	cases := []struct {
		urlPath string
		label   string
	}{
		{"/opportunity/id-with-bang!", "exclamation mark"},
		{"/opportunity/id%2Fpath", "percent-encoded slash"},
		{"/opportunity/id%3Bsemicolon", "percent-encoded semicolon"},
	}

	for _, tc := range cases {
		req := httptest.NewRequest("GET", tc.urlPath, http.NoBody)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("%s (%s): status = %d, want 404", tc.label, tc.urlPath, rr.Code)
		}
	}
}

// TestHandleDetail_AllSections verifies the detail page renders every required section.
func TestHandleDetail_AllSections(t *testing.T) {
	h, _ := newDetailHandler(t)

	req := httptest.NewRequest("GET", "/opportunity/opp-detail-1", http.NoBody)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	body := rr.Body.String()
	wantContent := []string{
		// navigation
		"Back to pipeline",
		// sections
		"Identification",
		"Dates",
		"Classification",
		"Description",
		"Scoring",
		"Eligibility",
		"Proposal Status",
		// eligibility placeholder id (required by spec)
		`id="eligibility-placeholder"`,
		// field values from seeded opportunity
		"opp-detail-1",
		"Test Federal Contract",
		"Department of Defense",
		"Army Corps",
		"W52P1J-26-R-0001",
		"541511",
		"Custom Computer Programming",
		"SBA",
		"Washington, DC",
		"Solicitation",
		"Firm Fixed Price",
		"View on SAM.gov",
		"This is the full opportunity description.",
		"87.3%",
		"Clearance required",
		"Strong alignment with capability profile.",
		"In Proposal",
		"Yes",
		"outline:success",
	}

	for _, want := range wantContent {
		if !strings.Contains(body, want) {
			t.Errorf("body missing expected content %q", want)
		}
	}
}

// TestHandleDetail_BIDRecommendation verifies BID is rendered with the green CSS class.
func TestHandleDetail_BIDRecommendation(t *testing.T) {
	h, _ := newDetailHandler(t)

	req := httptest.NewRequest("GET", "/opportunity/opp-detail-1", http.NoBody)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "rec-bid") {
		t.Errorf("body missing rec-bid CSS class for BID recommendation")
	}
}

// TestHandleDetail_DeadlineSoon verifies the deadline warning appears when the deadline is
// within 7 days.
func TestHandleDetail_DeadlineSoon(t *testing.T) {
	h, _ := newDetailHandler(t) // seeded opp has deadline 5 days away

	req := httptest.NewRequest("GET", "/opportunity/opp-detail-1", http.NoBody)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	body := rr.Body.String()
	// Spec requires a visual flag (⚠) or deadline-soon styling for near deadlines.
	if !strings.Contains(body, "⚠") && !strings.Contains(body, "deadline-soon") && !strings.Contains(body, "#fff0f0") {
		t.Errorf("body missing deadline warning for a deadline 5 days away; body excerpt: %s", body[:min(500, len(body))])
	}
}

// TestHandleDetail_NOBIDRecommendation verifies NO_BID is rendered with the red CSS class.
func TestHandleDetail_NOBIDRecommendation(t *testing.T) {
	s := newTestStore(t)
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	scoredAt := now

	opp := &opportunity.Opportunity{
		ID:             "opp-nobid",
		Title:          "No Bid Contract",
		Score:          0.2,
		ScoredAt:       &scoredAt,
		Recommendation: "NO_BID",
		UpdatedAt:      now,
	}
	if err := s.Save(context.Background(), opp); err != nil {
		t.Fatalf("Save: %v", err)
	}

	h := dashboard.NewHandler(dashboard.NewService(s))
	req := httptest.NewRequest("GET", "/opportunity/opp-nobid", http.NoBody)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "rec-nobid") {
		t.Errorf("body missing rec-nobid CSS class for NO_BID recommendation")
	}
}

// TestHandleList_LinksToDetail verifies that table rows in the list view link to the
// detail page for each opportunity.
func TestHandleList_LinksToDetail(t *testing.T) {
	h, opp := newDetailHandler(t)

	req := httptest.NewRequest("GET", "/", http.NoBody)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	wantLink := "/opportunity/" + opp.ID
	if !strings.Contains(body, wantLink) {
		t.Errorf("list body missing link %q to detail page", wantLink)
	}
}

// TestHandleList_Default checks that the list page renders without errors when the store
// has data.
func TestHandleList_Default(t *testing.T) {
	h, _ := newDetailHandler(t)

	req := httptest.NewRequest("GET", "/", http.NoBody)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{"Kaimi Pipeline", "Test Federal Contract", "Department of Defense"} {
		if !strings.Contains(body, want) {
			t.Errorf("list body missing %q", want)
		}
	}
}
