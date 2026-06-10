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

func TestHandleList(t *testing.T) {
	ctx := context.Background()
	s, err := store.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)

	// Seed some opportunities
	opps := []*opportunity.Opportunity{
		{
			ID:               "soon",
			Title:            "Deadline Soon",
			Agency:           "Agency A",
			Score:            0.9,
			ScoredAt:         &now,
			ResponseDeadline: now.Add(2 * 24 * time.Hour),
			UpdatedAt:        now,
		},
		{
			ID:               "late",
			Title:            "Deadline Late",
			Agency:           "Agency B",
			Score:            0.5,
			ScoredAt:         &now,
			ResponseDeadline: now.Add(10 * 24 * time.Hour),
			UpdatedAt:        now,
		},
		{
			ID:        "hunted",
			Title:     "Not Scored",
			Agency:    "Agency C",
			Score:     0,
			ScoredAt:  nil,
			UpdatedAt: now,
		},
	}
	for _, opp := range opps {
		if err := s.Save(ctx, opp); err != nil {
			t.Fatalf("failed to seed opportunity: %v", err)
		}
	}

	svc := dashboard.NewService(s)
	// We'll define NewHandler in handler.go
	h := dashboard.NewHandler(svc)
	h.Now = func() time.Time { return now }

	tests := []struct {
		name          string
		query         string
		wantStatus    int
		containsTexts []string
		excludesTexts []string
	}{
		{
			name:       "default list",
			query:      "",
			wantStatus: http.StatusOK,
			containsTexts: []string{
				"Deadline Soon",
				"Deadline Late",
				"Not Scored",
				"deadline-soon", // visual flag class
			},
		},
		{
			name:       "filter by stage",
			query:      "?stage=Scored",
			wantStatus: http.StatusOK,
			containsTexts: []string{
				"Deadline Soon",
				"Deadline Late",
			},
			excludesTexts: []string{
				"Not Scored",
			},
		},
		{
			name:       "filter by minScore",
			query:      "?minScore=0.8",
			wantStatus: http.StatusOK,
			containsTexts: []string{
				"Deadline Soon",
			},
			excludesTexts: []string{
				"Deadline Late",
				"Not Scored",
			},
		},
		{
			name:       "sort by score",
			query:      "?sort=score",
			wantStatus: http.StatusOK,
			containsTexts: []string{
				"Deadline Soon",
				"Deadline Late",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/"+tc.query, http.NoBody)
			rr := httptest.NewRecorder()

			h.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("got status %v, want %v", rr.Code, tc.wantStatus)
			}

			body := rr.Body.String()
			for _, text := range tc.containsTexts {
				if !contains(body, text) {
					t.Errorf("body missing expected text %q", text)
				}
			}
			for _, text := range tc.excludesTexts {
				if contains(body, text) {
					t.Errorf("body contains unexpected text %q", text)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return (len(s) >= len(substr)) && (func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}

// TestHandleListAdoptsDesignSystem verifies the overview layout consumes the
// brand and design-system assets (GitHub issue #141) instead of the
// pre-handoff placeholder styling.
func TestHandleListAdoptsDesignSystem(t *testing.T) {
	ctx := context.Background()
	s, err := store.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	if err := s.Save(ctx, &opportunity.Opportunity{
		ID: "one", Title: "Sample", Agency: "Agency", UpdatedAt: now,
	}); err != nil {
		t.Fatalf("failed to seed opportunity: %v", err)
	}

	h := dashboard.NewHandler(dashboard.NewService(s))
	h.Now = func() time.Time { return now }

	req := httptest.NewRequest("GET", "/", http.NoBody)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("got status %v, want %v", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()

	for _, want := range []string{
		`rel="icon"`,         // brand favicon (FaviconLink)
		"--st-human:",        // design-system tokens present (StyleTag)
		"#E8870E",            // the needs-human amber is defined
		"THE SEEKER",         // header lockup (HeaderLockup)
		"var(--st-human-bg)", // page styles expressed in token variables
	} {
		if !contains(body, want) {
			t.Errorf("overview body missing design-system marker %q", want)
		}
	}

	for _, ban := range []string{
		"#fffbe6", "#f0c040", "#0057b8", "#fff0f0", // placeholder palette
		"<h1>Kaimi Pipeline</h1>", // replaced by the lockup
	} {
		if contains(body, ban) {
			t.Errorf("overview body still contains placeholder styling %q", ban)
		}
	}
}

// seedDetailOpp is a fully-populated opportunity for detail-page tests.
func seedDetailOpp(t *testing.T, s store.Store, now time.Time) *opportunity.Opportunity {
	t.Helper()
	opp := &opportunity.Opportunity{
		ID:                 "ztamod-001",
		Title:              "Zero Trust Architecture Modernization",
		SolicitationNum:    "70RCSA24R0123",
		Agency:             "Dept. of Homeland Security",
		Office:             "CISA",
		PostedDate:         now.Add(-10 * 24 * time.Hour),
		ResponseDeadline:   now.Add(9 * 24 * time.Hour),
		NAICSCode:          "541512",
		NAICSDescription:   "Computer Systems Design Services",
		SetAsideCode:       "SBA",
		PlaceOfPerformance: "Washington, DC",
		Description:        "Modernize the agency's zero trust architecture.",
		Type:               "Solicitation",
		ContractType:       "Firm Fixed Price",
		URL:                "https://sam.gov/opp/ztamod-001",
		Score:              0.82,
		ScoreReasoning:     "Strong past performance in cybersecurity.",
		Recommendation:     "BID",
		Requirements:       []string{"FedRAMP High", "Top Secret facility clearance"},
		ScoredAt:           &now,
		CreatedAt:          now.Add(-10 * 24 * time.Hour),
		UpdatedAt:          now,
	}
	if err := s.Save(context.Background(), opp); err != nil {
		t.Fatalf("failed to seed opportunity: %v", err)
	}
	return opp
}

// TestHandleDetail verifies the /opportunity/{id} page (GitHub issue #111).
func TestHandleDetail(t *testing.T) {
	s, err := store.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	seedDetailOpp(t, s, now)

	h := dashboard.NewHandler(dashboard.NewService(s))
	h.Now = func() time.Time { return now }

	t.Run("valid id renders the full record", func(t *testing.T) {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", "/opportunity/ztamod-001", http.NoBody))
		if rr.Code != http.StatusOK {
			t.Fatalf("got status %v, want %v", rr.Code, http.StatusOK)
		}
		body := rr.Body.String()
		for _, want := range []string{
			"Zero Trust Architecture Modernization",
			"Dept. of Homeland Security",
			"70RCSA24R0123",
			"Computer Systems Design Services",
			"Modernize the agency&#39;s zero trust architecture.",
			"Strong past performance in cybersecurity.",
			"FedRAMP High",
			"82.0%",                        // ScoreDisplay
			`class="kfit"`,                 // FitRing (design system)
			"krec--bid",                    // RecommendationPill
			"kdead--near",                  // DeadlinePill at 9 days
			`class="ktag"`,                 // MetaTag for NAICS/SOL
			`id="eligibility-placeholder"`, // Phase 1+ placeholder per ux-spec
			"Back to pipeline",             // navigation
			"View on SAM.gov",              // solicitation link
			"Scored",                       // derived stage
			`http-equiv="refresh"`,         // live page keeps auto-refresh
			"THE SEEKER",                   // shared branded layout
		} {
			if !contains(body, want) {
				t.Errorf("detail body missing %q", want)
			}
		}
	})

	t.Run("unknown id returns 404 without refresh", func(t *testing.T) {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", "/opportunity/nope", http.NoBody))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("got status %v, want %v", rr.Code, http.StatusNotFound)
		}
		body := rr.Body.String()
		if !contains(body, "Opportunity not found: nope") {
			t.Errorf("404 body missing not-found message, got:\n%s", body)
		}
		if contains(body, `http-equiv="refresh"`) {
			t.Errorf("404 page must not auto-refresh (ux-spec)")
		}
	})

	t.Run("invalid id characters are rejected with 404", func(t *testing.T) {
		for _, id := range []string{"bad%24id", "a%20b", "%2e%2e", "x%3Cscript%3E"} {
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, httptest.NewRequest("GET", "/opportunity/"+id, http.NoBody))
			if rr.Code != http.StatusNotFound {
				t.Errorf("id %q: got status %v, want 404", id, rr.Code)
			}
			if contains(rr.Body.String(), "<script>") {
				t.Errorf("id %q: unescaped input reflected in response", id)
			}
		}
	})

	t.Run("unscored opportunity shows dashes and no ring", func(t *testing.T) {
		if err := s.Save(context.Background(), &opportunity.Opportunity{
			ID: "raw-1", Title: "Unscored Opp", Agency: "GSA", UpdatedAt: now,
		}); err != nil {
			t.Fatalf("failed to seed: %v", err)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", "/opportunity/raw-1", http.NoBody))
		if rr.Code != http.StatusOK {
			t.Fatalf("got status %v, want %v", rr.Code, http.StatusOK)
		}
		body := rr.Body.String()
		if contains(body, `class="kfit"`) {
			t.Errorf("unscored detail should not render a fit ring")
		}
		if !contains(body, "Hunted") {
			t.Errorf("unscored detail should show the Hunted stage")
		}
	})
}

// TestListLinksToDetail verifies table rows link to their detail page (#111).
func TestListLinksToDetail(t *testing.T) {
	s, err := store.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	seedDetailOpp(t, s, now)

	h := dashboard.NewHandler(dashboard.NewService(s))
	h.Now = func() time.Time { return now }

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", http.NoBody))
	if !contains(rr.Body.String(), `<a href="/opportunity/ztamod-001">`) {
		t.Errorf("list table should link rows to their detail page")
	}
}
