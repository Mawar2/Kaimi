package desktop_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/desktop"
	"github.com/Mawar2/Kaimi/internal/document"
	"github.com/Mawar2/Kaimi/internal/finalreview"
	"github.com/Mawar2/Kaimi/internal/googledocs"
	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/outline"
	"github.com/Mawar2/Kaimi/internal/proposal"
	"github.com/Mawar2/Kaimi/internal/store"
	"github.com/Mawar2/Kaimi/internal/writer"
)

// newStubProposals builds a real proposal.Service over the given store dir with
// the cached Outline docs client + stub Writer + deterministic Final Review —
// the same offline wiring the web handler tests use, so the desktop backend is
// exercised against the real Zone-2 lifecycle minus only the live LLM.
func newStubProposals(t *testing.T, dir string) *proposal.Service {
	t.Helper()
	opps, err := store.NewJSONStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	docs, err := document.NewStore(dir)
	if err != nil {
		t.Fatalf("docs: %v", err)
	}
	docsClient, err := googledocs.NewClient(context.Background(), googledocs.Config{UseCached: true})
	if err != nil {
		t.Fatalf("docs client: %v", err)
	}
	return proposal.NewService(&proposal.Deps{
		Opportunities: opps, Documents: docs,
		Outline: outline.New(docsClient), Writer: writer.New(), Review: finalreview.New(),
	})
}

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

// TestSelectAndListProposals proves the desktop backend drives the real Zone-2
// lifecycle (issue #249): Select runs the draft pipeline to the human gate, and
// ListProposals reports the proposal with the SAME state derivation the web uses
// (internal/zone2view) — so the desktop and web cannot disagree (B2).
func TestSelectAndListProposals(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	seedStore(t, dir, &opportunity.Opportunity{
		ID: "zta-1", Title: "Zero Trust Architecture Modernization",
		Agency: "DHS CISA", NAICSCode: "541512",
		Description:      "Modernize zero trust architecture.",
		ResponseDeadline: now.Add(20 * 24 * time.Hour),
		Score:            0.87, Recommendation: "BID",
		Requirements: []string{"FedRAMP High"},
		ScoredAt:     &now, CreatedAt: now, UpdatedAt: now,
	})
	proposals := newStubProposals(t, dir)
	b, err := desktop.New(dir, desktop.WithProposals(proposals))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Before selection there are no proposals.
	if res, err := b.ListProposals(context.Background()); err != nil || !res.Empty {
		t.Fatalf("expected empty proposals before select (err=%v empty=%v)", err, res.Empty)
	}

	// Select runs the pipeline to the gate.
	if err := b.Select(context.Background(), "zta-1"); err != nil {
		t.Fatalf("Select: %v", err)
	}
	proposals.Wait()

	res, err := b.ListProposals(context.Background())
	if err != nil {
		t.Fatalf("ListProposals: %v", err)
	}
	if res.Empty || len(res.Cards) != 1 {
		t.Fatalf("expected 1 proposal card, got empty=%v len=%d", res.Empty, len(res.Cards))
	}
	card := res.Cards[0]
	if card.State != "human" {
		t.Errorf("card state = %q, want human (at the gate)", card.State)
	}
	if card.When != "Paused for your review" {
		t.Errorf("card phrase = %q, want 'Paused for your review'", card.When)
	}
	if res.NeedsYou != 1 {
		t.Errorf("NeedsYou = %d, want 1", res.NeedsYou)
	}
}

// TestProposalActionsRequireService keeps a read-only backend valid: the
// mutating methods report a clear error rather than panicking when no proposal
// service is wired.
func TestProposalActionsRequireService(t *testing.T) {
	b, err := desktop.New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := b.Select(context.Background(), "x"); err == nil {
		t.Errorf("Select without a proposal service should error")
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
