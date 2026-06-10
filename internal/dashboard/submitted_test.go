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
	"github.com/Mawar2/Kaimi/internal/store"
)

// TestHandleSubmitted verifies the Submitted archive (design PIPELINE.md §3):
// submitted proposals appear with their reference documents, and opportunities
// that have not been submitted do not.
func TestHandleSubmitted(t *testing.T) {
	ctx := context.Background()
	s, err := store.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	submitted := &opportunity.Opportunity{
		ID: "sub-1", Title: "Submitted Proposal", Agency: "Agency S",
		SolicitationNum: "SOL-123", URL: "https://sam.gov/opp/sub-1/view",
		Score: 0.8, ScoredAt: &now, Selected: true, SelectedAt: &now,
		ProposalStatus: "submitted", UpdatedAt: now,
	}
	queued := &opportunity.Opportunity{
		ID: "q-1", Title: "Queued Opportunity", Agency: "Agency Q",
		Score: 0.7, ScoredAt: &now, UpdatedAt: now,
	}
	for _, o := range []*opportunity.Opportunity{submitted, queued} {
		if err := s.Save(ctx, o); err != nil {
			t.Fatalf("seed %s: %v", o.ID, err)
		}
	}

	h := dashboard.NewHandler(dashboard.NewService(s))
	h.Now = func() time.Time { return now }

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/submitted", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /submitted status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Submitted Proposal") {
		t.Errorf("submitted proposal missing from the archive")
	}
	if strings.Contains(body, "Queued Opportunity") {
		t.Errorf("an un-submitted opportunity must not appear in the archive")
	}
	if !strings.Contains(body, "View solicitation") {
		t.Errorf("reference-document (solicitation) link missing")
	}
	if !strings.Contains(body, "Working draft") {
		t.Errorf("reference-document (working draft) link missing")
	}
}
