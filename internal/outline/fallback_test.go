package outline

import (
	"context"
	"errors"
	"testing"
)

func TestFallbackPlanner_PrimaryFails_FallsBackToDeterministic(t *testing.T) {
	opp := plannerTestOpp()
	source := combinedSource(opp, nil)

	// Primary always errors; the appended deterministic planner must still produce
	// the rule-based sections (it never fails).
	fp := NewFallbackPlanner(&fakePlanner{err: errors.New("flash down")})
	got, err := fp.PlanSections(context.Background(), opp, source)
	if err != nil {
		t.Fatalf("fallback should succeed via the deterministic planner, got: %v", err)
	}
	want := buildSections(opp, source)
	if len(got) != len(want) || len(got) == 0 {
		t.Fatalf("got %d sections, want the deterministic %d", len(got), len(want))
	}
}

func TestFallbackPlanner_PrimarySucceeds_UsesPrimary(t *testing.T) {
	primary := &fakePlanner{sections: []Section{{ID: "x", Title: "From Primary"}}}
	got, err := NewFallbackPlanner(primary).PlanSections(context.Background(), plannerTestOpp(), "src")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(got) != 1 || got[0].Title != "From Primary" {
		t.Errorf("got %+v, want the primary planner's sections", got)
	}
}

func TestFallbackPlanner_PrimaryEmpty_FallsThrough(t *testing.T) {
	// A planner that returns no sections (no error) is skipped in favor of the
	// next planner — here the always-appended deterministic one.
	opp := plannerTestOpp()
	empty := &fakePlanner{sections: nil}
	got, err := NewFallbackPlanner(empty).PlanSections(context.Background(), opp, combinedSource(opp, nil))
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected the deterministic planner to supply sections when the primary is empty")
	}
}
