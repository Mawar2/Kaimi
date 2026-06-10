package eval

import (
	"path/filepath"
	"testing"

	"github.com/Mawar2/Kaimi/internal/scorer"
)

// TestLoadScorerCases reads the seed scorer dataset from disk, resolves each case's
// fixture reference into an Opportunity, and verifies the labels round-trip.
func TestLoadScorerCases(t *testing.T) {
	dir := filepath.Join("..", "..", "test", "fixtures", "eval")
	cases, err := LoadScorerCases(filepath.Join(dir, "scorer_cases.json"), dir)
	if err != nil {
		t.Fatalf("LoadScorerCases: %v", err)
	}
	if len(cases) < 4 {
		t.Fatalf("expected at least 4 seed scorer cases, got %d", len(cases))
	}
	for _, c := range cases {
		if c.Name == "" {
			t.Error("case has empty Name")
		}
		if c.Opportunity == nil {
			t.Errorf("case %q has nil Opportunity (fixture not resolved)", c.Name)
		}
		switch c.ExpectedRecommendation {
		case scorer.RecommendationBID, scorer.RecommendationNoBid, scorer.RecommendationReview:
			// valid
		default:
			t.Errorf("case %q has invalid ExpectedRecommendation %q", c.Name, c.ExpectedRecommendation)
		}
	}
}

// TestLoadWriterCases reads the seed writer dataset and verifies each case carries a
// prompt and at least one fact.
func TestLoadWriterCases(t *testing.T) {
	dir := filepath.Join("..", "..", "test", "fixtures", "eval")
	cases, err := LoadWriterCases(filepath.Join(dir, "writer_cases.json"))
	if err != nil {
		t.Fatalf("LoadWriterCases: %v", err)
	}
	if len(cases) < 4 {
		t.Fatalf("expected at least 4 seed writer cases, got %d", len(cases))
	}
	for _, c := range cases {
		if c.SectionPrompt == "" {
			t.Errorf("case %q has empty SectionPrompt", c.Name)
		}
		if len(c.Facts) == 0 {
			t.Errorf("case %q has no Facts", c.Name)
		}
	}
}
