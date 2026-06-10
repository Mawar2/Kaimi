package dashboard_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/dashboard"
	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/store"
)

func seedSubmitted(t *testing.T, s store.Store, now time.Time) {
	t.Helper()
	sub := func(id, title, agency string, val float64, outcome string) *opportunity.Opportunity {
		when := now.Add(-30 * 24 * time.Hour)
		return &opportunity.Opportunity{
			ID: id, Title: title, Agency: agency, SolicitationNum: "SOL-" + id,
			Score: 0.8, Selected: true, ProposalStatus: "submitted",
			EstimatedValue: val, SubmittedAt: &when, AwardOutcome: outcome,
			CreatedAt: when, UpdatedAt: when,
		}
	}
	for _, o := range []*opportunity.Opportunity{
		sub("won1", "ICAM Modernization", "GSA", 1_800_000, "won"),
		sub("pend1", "Zero Trust Architecture", "DHS CISA", 3_200_000, ""),
		sub("lost1", "Secure SD-WAN", "USDA", 1_400_000, "lost"),
	} {
		if err := s.Save(context.Background(), o); err != nil {
			t.Fatalf("seed %s: %v", o.ID, err)
		}
	}
}

func TestSubmittedArchive(t *testing.T) {
	s, err := store.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	seedSubmitted(t, s, now)
	h := dashboard.NewHandler(dashboard.NewService(s))
	h.Now = func() time.Time { return now }

	get := func(path string) string {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", path, http.NoBody))
		if rr.Code != http.StatusOK {
			t.Fatalf("GET %s: status %d", path, rr.Code)
		}
		return rr.Body.String()
	}

	t.Run("renders the archive with value stats and outcome badges", func(t *testing.T) {
		body := get("/submitted")
		for _, want := range []string{
			"<h1>Submitted</h1>",
			"ICAM Modernization", "Zero Trust Architecture", "Secure SD-WAN",
			"$3.2M",                     // pending value stat
			"$1.8M",                     // won value stat
			"kbadge--done",              // Won badge
			"kbadge--muted",             // Not awarded badge
			"kbadge--pending",           // Pending award badge
			`<details class="srow">`,    // no-JS expandable rows
			`name="status" value="won"`, // segmented filter button (not a raw link)
		} {
			if !contains(body, want) {
				t.Errorf("/submitted missing %q", want)
			}
		}
	})

	t.Run("status filter narrows the list", func(t *testing.T) {
		body := get("/submitted?status=won")
		if !contains(body, "ICAM Modernization") || contains(body, "Secure SD-WAN") {
			t.Errorf("status=won should show only the won proposal")
		}
	})

	t.Run("search filters by title/agency/solicitation", func(t *testing.T) {
		body := get("/submitted?q=SD-WAN")
		if !contains(body, "Secure SD-WAN") || contains(body, "ICAM Modernization") {
			t.Errorf("q=SD-WAN should show only the matching proposal")
		}
	})
}
