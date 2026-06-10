package proposal

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/agent"
	"github.com/Mawar2/Kaimi/internal/document"
	"github.com/Mawar2/Kaimi/internal/finalreview"
	"github.com/Mawar2/Kaimi/internal/googledocs"
	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/outline"
	"github.com/Mawar2/Kaimi/internal/scorer"
	"github.com/Mawar2/Kaimi/internal/store"
	"github.com/Mawar2/Kaimi/internal/writer"
)

// newTestService wires the REAL agents end to end: the real Outline agent
// with the cached (no-network) Google Docs client, the real Writer in stub
// mode, and the real Final Review agent. Only the LLM is absent.
func newTestService(t *testing.T) (*Service, store.Store) {
	t.Helper()
	dir := t.TempDir()
	opps, err := store.NewJSONStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	docs, err := document.NewStore(dir)
	if err != nil {
		t.Fatalf("document store: %v", err)
	}
	docsClient, err := googledocs.NewClient(context.Background(), googledocs.Config{UseCached: true})
	if err != nil {
		t.Fatalf("googledocs cached client: %v", err)
	}
	svc := NewService(&Deps{
		Opportunities: opps,
		Documents:     docs,
		Outline:       outline.New(docsClient),
		Writer:        writer.New(), // stub mode: deterministic, no LLM
		Review:        finalreview.New(),
		Profile:       &scorer.CapabilityProfile{},
	})
	return svc, opps
}

func seedOpp(t *testing.T, s store.Store) *opportunity.Opportunity {
	t.Helper()
	now := time.Now()
	opp := &opportunity.Opportunity{
		ID:               "zta-1",
		Title:            "Zero Trust Architecture Modernization",
		Agency:           "DHS CISA",
		NAICSCode:        "541512",
		Description:      "Modernize zero trust architecture.",
		ResponseDeadline: now.Add(30 * 24 * time.Hour),
		Score:            0.87,
		Recommendation:   "BID",
		Requirements:     []string{"FedRAMP High"},
		ScoredAt:         &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.Save(context.Background(), opp); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return opp
}

func TestSelectRunsRealAgentsToTheGate(t *testing.T) {
	svc, opps := newTestService(t)
	seedOpp(t, opps)

	if err := svc.Select(context.Background(), "zta-1"); err != nil {
		t.Fatalf("Select: %v", err)
	}
	svc.Wait()

	opp, err := opps.Get(context.Background(), "zta-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !opp.Selected || opp.SelectedAt == nil {
		t.Errorf("opportunity not marked selected")
	}
	if opp.ProposalStatus != StatusGate {
		t.Fatalf("ProposalStatus = %q, want %q (the pipeline must PAUSE at the human gate, never run final review)", opp.ProposalStatus, StatusGate)
	}

	doc, err := svc.Document("zta-1")
	if err != nil {
		t.Fatalf("Document: %v", err)
	}
	if len(doc.Sections) < 5 {
		t.Errorf("outline should produce at least the five standard volumes, got %d", len(doc.Sections))
	}
	for _, sec := range doc.Sections {
		if strings.TrimSpace(sec.Body) == "" {
			t.Errorf("section %q has no drafted body", sec.ID)
		}
	}
	actors := []string{}
	for _, r := range doc.Revisions {
		actors = append(actors, r.Actor)
	}
	if len(actors) < 2 || actors[0] != "outline" || actors[1] != "writer" {
		t.Errorf("revision trail should be outline then writer, got %v", actors)
	}
}

func TestSelectTwiceFails(t *testing.T) {
	svc, opps := newTestService(t)
	seedOpp(t, opps)
	if err := svc.Select(context.Background(), "zta-1"); err != nil {
		t.Fatalf("Select: %v", err)
	}
	svc.Wait()
	if err := svc.Select(context.Background(), "zta-1"); err == nil {
		t.Errorf("second Select must fail")
	}
}

func TestApproveRunsRealFinalReview_FindsGaps(t *testing.T) {
	svc, opps := newTestService(t)
	seedOpp(t, opps)
	if err := svc.Select(context.Background(), "zta-1"); err != nil {
		t.Fatalf("Select: %v", err)
	}
	svc.Wait()

	// The stub draft does not mention "FedRAMP High", so the real Final
	// Review agent must send it back to the human with flags.
	if err := svc.Approve(context.Background(), "zta-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	svc.Wait()

	opp, _ := opps.Get(context.Background(), "zta-1")
	if opp.ProposalStatus != "final-review:needs_human" {
		t.Fatalf("ProposalStatus = %q, want final-review:needs_human", opp.ProposalStatus)
	}
	doc, _ := svc.Document("zta-1")
	if len(doc.Flags) == 0 {
		t.Errorf("final review issues should land as document flags")
	}
}

func TestHumanEditsAreWhatVeraReviews(t *testing.T) {
	svc, opps := newTestService(t)
	seedOpp(t, opps)
	if err := svc.Select(context.Background(), "zta-1"); err != nil {
		t.Fatalf("Select: %v", err)
	}
	svc.Wait()

	// Human edits the draft at the gate to satisfy the must-have
	// requirement; Final Review must run on THIS revision (INTENT.md).
	doc, _ := svc.Document("zta-1")
	if _, err := svc.UpdateSection(context.Background(), "zta-1", doc.Sections[0].ID,
		"We will use FedRAMP High authorized tooling throughout."); err != nil {
		t.Fatalf("UpdateSection: %v", err)
	}

	if err := svc.Approve(context.Background(), "zta-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	svc.Wait()

	opp, _ := opps.Get(context.Background(), "zta-1")
	if opp.ProposalStatus != "final-review:ready_to_submit" {
		t.Fatalf("ProposalStatus = %q, want final-review:ready_to_submit (review must pass on the human-edited revision)", opp.ProposalStatus)
	}

	// Submit is a human act and only valid from ready_to_submit.
	if err := svc.Submit(context.Background(), "zta-1"); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	opp, _ = opps.Get(context.Background(), "zta-1")
	if opp.ProposalStatus != StatusSubmitted {
		t.Errorf("ProposalStatus = %q, want %q", opp.ProposalStatus, StatusSubmitted)
	}
}

func TestRequestChangesLoopsBackToGate(t *testing.T) {
	svc, opps := newTestService(t)
	seedOpp(t, opps)
	if err := svc.Select(context.Background(), "zta-1"); err != nil {
		t.Fatalf("Select: %v", err)
	}
	svc.Wait()

	if err := svc.RequestChanges(context.Background(), "zta-1", "Tighten the technical approach."); err != nil {
		t.Fatalf("RequestChanges: %v", err)
	}
	svc.Wait()

	opp, _ := opps.Get(context.Background(), "zta-1")
	if opp.ProposalStatus != StatusGate {
		t.Fatalf("ProposalStatus = %q, want back at %q", opp.ProposalStatus, StatusGate)
	}
	doc, _ := svc.Document("zta-1")
	found := false
	for _, r := range doc.Revisions {
		if strings.Contains(r.Note, "Tighten the technical approach.") {
			found = true
		}
	}
	if !found {
		t.Errorf("the human's change-request note must be recorded in the revision history")
	}
}

func TestGuards(t *testing.T) {
	svc, opps := newTestService(t)
	seedOpp(t, opps)

	if err := svc.Approve(context.Background(), "zta-1"); err == nil {
		t.Errorf("Approve before the gate must fail")
	}
	if err := svc.Submit(context.Background(), "zta-1"); err == nil {
		t.Errorf("Submit before ready_to_submit must fail")
	}
	if _, err := svc.UpdateSection(context.Background(), "zta-1", "x", "y"); err == nil {
		t.Errorf("UpdateSection without a document must fail")
	}
	if err := svc.Select(context.Background(), "missing"); err == nil {
		t.Errorf("Select on unknown opportunity must fail")
	}
}

// recordingWriter records every Run call so tests can prove section-by-
// section drafting (issue #158).
type recordingWriter struct {
	mu    sync.Mutex
	calls []writerCall
}

type writerCall struct {
	sectionCount int
	title        string
}

func (r *recordingWriter) Run(_ context.Context, in writer.Input) (string, *agent.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	title := ""
	if len(in.Outline.Sections) > 0 {
		title = in.Outline.Sections[0].Title
	}
	r.calls = append(r.calls, writerCall{sectionCount: len(in.Outline.Sections), title: title})
	draft := "\n## " + title + "\nDrafted body for " + title + "\n"
	return draft, &agent.Result{AgentName: "writer", Status: agent.StatusSuccess, CompletedAt: time.Now()}, nil
}

// TestWriterDraftsSectionBySection proves the document grows incrementally:
// one Writer run per outline section, applied as each completes, so the
// human can review the outline (and early sections) while drafting runs.
func TestWriterDraftsSectionBySection(t *testing.T) {
	dir := t.TempDir()
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
	rec := &recordingWriter{}
	svc := NewService(&Deps{
		Opportunities: opps,
		Documents:     docs,
		Outline:       outline.New(docsClient),
		Writer:        rec,
		Review:        finalreview.New(),
		Profile:       &scorer.CapabilityProfile{},
	})
	seedOpp(t, opps)

	if err := svc.Select(context.Background(), "zta-1"); err != nil {
		t.Fatalf("Select: %v", err)
	}
	svc.Wait()

	doc, err := svc.Document("zta-1")
	if err != nil {
		t.Fatalf("Document: %v", err)
	}
	if len(rec.calls) != len(doc.Sections) {
		t.Fatalf("writer ran %d times for %d sections — want one run per section", len(rec.calls), len(doc.Sections))
	}
	for _, c := range rec.calls {
		if c.sectionCount != 1 {
			t.Errorf("each writer run must receive a single-section outline, got %d (%q)", c.sectionCount, c.title)
		}
	}
	for _, sec := range doc.Sections {
		if !strings.Contains(sec.Body, "Drafted body for "+sec.Heading) {
			t.Errorf("section %q body not applied from its own run", sec.ID)
		}
	}
	// Incremental application means one writer revision per section.
	writerRevs := 0
	for _, r := range doc.Revisions {
		if r.Actor == "writer" {
			writerRevs++
		}
	}
	if writerRevs != len(doc.Sections) {
		t.Errorf("want %d writer revisions (one per section), got %d", len(doc.Sections), writerRevs)
	}
}
