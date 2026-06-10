package main

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

// newTestMux seeds a store with one opportunity per early stage and returns
// the production route table (newMux) serving it.
func newTestMux(t *testing.T) *http.ServeMux {
	t.Helper()
	s, err := store.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	opps := []opportunity.Opportunity{
		{ID: "opp1", Title: "Hunted Opp", UpdatedAt: now},
		{ID: "opp2", Title: "Scored Opp", ScoredAt: &now, Score: 0.8, UpdatedAt: now},
		{ID: "opp3", Title: "Selected Opp", Selected: true, UpdatedAt: now},
	}
	for i := range opps {
		if err := s.Save(context.Background(), &opps[i]); err != nil {
			t.Fatal(err)
		}
	}
	return newMux(dashboard.NewHandler(dashboard.NewService(s)))
}

// TestRouting exercises the composed route table — the integration seam that
// issue #147 found broken (detail links 404ing, duplicate unbranded overview).
func TestRouting(t *testing.T) {
	mux := newTestMux(t)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		contains   []string
		excludes   []string
	}{
		{
			name:       "root serves the branded overview with cards and table",
			path:       "/",
			wantStatus: http.StatusOK,
			contains: []string{
				"the seeker",               // branded lockup (CSS uppercases)
				"in queue",                 // Triage stat strip
				"Hunted Opp",               // …and the table on the same page
				`href="/opportunity/opp2"`, // detail links
			},
		},
		{
			name:       "detail route is reachable",
			path:       "/opportunity/opp2",
			wantStatus: http.StatusOK,
			contains:   []string{"Scored Opp", "All opportunities", "the seeker"},
		},
		{
			name:       "unknown opportunity renders the branded 404",
			path:       "/opportunity/missing",
			wantStatus: http.StatusNotFound,
			contains:   []string{"Opportunity not found: missing"},
		},
		{
			name:       "opportunities alias still serves the filterable list",
			path:       "/opportunities?stage=Scored",
			wantStatus: http.StatusOK,
			contains:   []string{"Scored Opp"},
			excludes:   []string{"Hunted Opp", "Selected Opp"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, tc.path, http.NoBody))
			if rr.Code != tc.wantStatus {
				t.Fatalf("GET %s: got status %v, want %v", tc.path, rr.Code, tc.wantStatus)
			}
			body := rr.Body.String()
			for _, want := range tc.contains {
				if !strings.Contains(body, want) {
					t.Errorf("GET %s: body missing %q", tc.path, want)
				}
			}
			for _, ban := range tc.excludes {
				if strings.Contains(body, ban) {
					t.Errorf("GET %s: body unexpectedly contains %q", tc.path, ban)
				}
			}
		})
	}
}

// TestNoDuplicateOverview pins the anti-bloat half of #147: the overview must
// come from the shared branded handler, not a second cmd-local template.
func TestNoDuplicateOverview(t *testing.T) {
	mux := newTestMux(t)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	body := rr.Body.String()
	if !strings.Contains(body, "--st-human") {
		t.Errorf("overview must carry the design-system tokens (branded handler)")
	}
	if strings.Contains(body, `class="stage-container"`) {
		t.Errorf("overview is still rendering the old cmd-local template")
	}
}
