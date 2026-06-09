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
	s, _ := store.NewJSONStore(t.TempDir())
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
