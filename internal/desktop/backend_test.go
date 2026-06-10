package desktop_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/desktop"
	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/store"
)

// seedStore creates a JSON store at basePath and saves the given opportunities.
func seedStore(t *testing.T, basePath string, opps ...*opportunity.Opportunity) {
	t.Helper()
	s, err := store.NewJSONStore(basePath)
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	for _, opp := range opps {
		if err := s.Save(context.Background(), opp); err != nil {
			t.Fatalf("Save(%s): %v", opp.ID, err)
		}
	}
}

func TestResolveStorePath_OverrideWins(t *testing.T) {
	t.Setenv("KAIMI_STORE_PATH", "C:/env/path")
	got, err := desktop.ResolveStorePath("C:/explicit/override")
	if err != nil {
		t.Fatalf("ResolveStorePath: %v", err)
	}
	if got != "C:/explicit/override" {
		t.Errorf("override should win, got %q", got)
	}
}

func TestResolveStorePath_EnvFallback(t *testing.T) {
	t.Setenv("KAIMI_STORE_PATH", "C:/env/path")
	got, err := desktop.ResolveStorePath("")
	if err != nil {
		t.Fatalf("ResolveStorePath: %v", err)
	}
	if got != "C:/env/path" {
		t.Errorf("env should be used when no override, got %q", got)
	}
}

func TestResolveStorePath_DefaultIsKaimiScoped(t *testing.T) {
	t.Setenv("KAIMI_STORE_PATH", "")
	got, err := desktop.ResolveStorePath("")
	if err != nil {
		t.Fatalf("ResolveStorePath: %v", err)
	}
	if got == "" {
		t.Fatal("default store path must not be empty")
	}
	if !strings.Contains(got, "Kaimi") {
		t.Errorf("default store path should be Kaimi-scoped, got %q", got)
	}
}

func TestNew_MissingDirDoesNotCrash(t *testing.T) {
	// A store path that does not exist yet must not crash; the JSON store
	// creates it. This is the "missing store shows empty state, not a crash" AC.
	missing := filepath.Join(t.TempDir(), "does", "not", "exist", "yet")
	b, err := desktop.New(missing)
	if err != nil {
		t.Fatalf("New on missing dir should succeed, got %v", err)
	}
	res, err := b.ListOpportunities(context.Background())
	if err != nil {
		t.Fatalf("ListOpportunities: %v", err)
	}
	if !res.Empty {
		t.Errorf("missing/empty store should report Empty=true")
	}
}

func TestListOpportunities_EmptyStoreFriendlyState(t *testing.T) {
	dir := t.TempDir()
	seedStore(t, dir) // creates the store with zero opportunities

	b, err := desktop.New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := b.ListOpportunities(context.Background())
	if err != nil {
		t.Fatalf("ListOpportunities: %v", err)
	}
	if !res.Empty {
		t.Errorf("Empty should be true for a store with no opportunities")
	}
	if len(res.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(res.Rows))
	}
	if strings.TrimSpace(res.Message) == "" {
		t.Errorf("empty state should carry a friendly, non-empty Message")
	}
}

func TestListOpportunities_ListsWithDerivedStage(t *testing.T) {
	dir := t.TempDir()
	scored := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	seedStore(t, dir,
		&opportunity.Opportunity{
			ID:       "OPP-HUNTED",
			Title:    "Freshly hunted",
			Agency:   "GSA",
			Selected: false, // not scored, not selected -> Hunted
		},
		&opportunity.Opportunity{
			ID:             "OPP-INPROP",
			Title:          "Being drafted",
			Agency:         "DoD",
			ScoredAt:       &scored,
			Selected:       true,
			ProposalStatus: "outline", // selected + non-empty status -> In Proposal
		},
	)

	b, err := desktop.New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := b.ListOpportunities(context.Background())
	if err != nil {
		t.Fatalf("ListOpportunities: %v", err)
	}
	if res.Empty {
		t.Errorf("Empty should be false when opportunities exist")
	}
	if len(res.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(res.Rows))
	}

	stageByID := map[string]string{}
	for _, r := range res.Rows {
		stageByID[r.ID] = string(r.Stage)
	}
	if stageByID["OPP-HUNTED"] != "Hunted" {
		t.Errorf("OPP-HUNTED stage = %q, want Hunted", stageByID["OPP-HUNTED"])
	}
	if stageByID["OPP-INPROP"] != "In Proposal" {
		t.Errorf("OPP-INPROP stage = %q, want In Proposal", stageByID["OPP-INPROP"])
	}
}
