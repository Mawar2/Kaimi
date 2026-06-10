package proposal

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Mawar2/Kaimi/internal/document"
	"github.com/Mawar2/Kaimi/internal/finalreview"
	"github.com/Mawar2/Kaimi/internal/manager"
	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/outline"
	"github.com/Mawar2/Kaimi/internal/scorer"
	"github.com/Mawar2/Kaimi/internal/store"
	"github.com/Mawar2/Kaimi/internal/writer"
)

// ProposalStatus values this service writes to Opportunity.ProposalStatus,
// extending the Manager's "{stage}:{status}" vocabulary with in-progress
// markers, the human gate, and the terminal human-submitted state.
const (
	// StatusOutlineRunning means Noa is building the document skeleton.
	StatusOutlineRunning = "outline:in_progress"
	// StatusWriterRunning means Tomás is drafting (or revising) sections.
	StatusWriterRunning = "writer:in_progress"
	// StatusGate is the single human review gate: the draft is paused for
	// the human to read, edit, and decide.
	StatusGate = "writer:needs_human"
	// StatusReviewRunning means Vera is running the final pass.
	StatusReviewRunning = "final-review:in_progress"
	// StatusReadyToSubmit means the final pass passed; a human may submit.
	StatusReadyToSubmit = "final-review:ready_to_submit"
	// StatusReviewNeedsHuman means the final pass found issues; the human
	// is back at the gate with flags.
	StatusReviewNeedsHuman = "final-review:needs_human"
	// StatusSubmitted is the terminal state, set only by the human Submit
	// action. Agents stand down.
	StatusSubmitted = "submitted"
)

// stageTimeout bounds each background agent stage.
const stageTimeout = 10 * time.Minute

// Deps wires the service. The agent fields take the SAME interfaces the
// Manager defines, so the real agents (and their stubs/mocks) drop in
// unchanged — this service deliberately does not modify internal/manager,
// internal/writer, internal/outline, or internal/finalreview.
type Deps struct {
	Opportunities store.Store
	Documents     *document.Store
	Outline       manager.OutlineRunner
	Writer        manager.WriterRunner
	Review        manager.Reviewer
	Profile       *scorer.CapabilityProfile
}

// Service drives the gated Zone 2 proposal lifecycle: Select starts the
// real agents and pauses at the single human gate; the human edits the
// document; Approve resumes with the real Final Review on the human-edited
// revision; Submit is always a human act. Both the web dashboard and the
// desktop app call this service — it is the shared backend of epic #153.
type Service struct {
	deps *Deps
	// Now is injected for deterministic tests; defaults to time.Now.
	Now func() time.Time

	mu      sync.Mutex
	running map[string]bool
	wg      sync.WaitGroup
}

// NewService validates deps and returns a Service.
func NewService(deps *Deps) *Service {
	return &Service{
		deps:    deps,
		Now:     time.Now,
		running: make(map[string]bool),
	}
}

// Wait blocks until all background agent stages have finished. Tests and
// graceful shutdown use it; handlers never need to.
func (s *Service) Wait() { s.wg.Wait() }

// Document returns the current proposal document for the opportunity.
func (s *Service) Document(oppID string) (*document.Document, error) {
	return s.deps.Documents.Get(oppID)
}

// Select is the bridge event from Zone 1: the human chooses to pursue an
// opportunity. It marks the selection and starts the draft pipeline (real
// Outline agent, then real Writer agent) in the background; the pipeline
// pauses at the human gate, never running Final Review on its own.
func (s *Service) Select(ctx context.Context, oppID string) error {
	opp, err := s.deps.Opportunities.Get(ctx, oppID)
	if err != nil {
		return fmt.Errorf("select %s: %w", oppID, err)
	}
	if opp.Selected {
		return fmt.Errorf("opportunity %s is already in your proposals", oppID)
	}
	if !s.claim(oppID) {
		return fmt.Errorf("opportunity %s already has a stage running", oppID)
	}

	now := s.Now()
	opp.Selected = true
	opp.SelectedAt = &now
	opp.ProposalStatus = StatusOutlineRunning
	opp.UpdatedAt = now
	if err := s.deps.Opportunities.Save(ctx, opp); err != nil {
		s.release(oppID)
		return fmt.Errorf("select %s: %w", oppID, err)
	}

	s.spawn(oppID, s.runDraftPipeline)
	return nil
}

// UpdateSection records a human edit to one document section. Edits are
// only meaningful while the proposal is paused at a gate.
func (s *Service) UpdateSection(ctx context.Context, oppID, sectionID, body string) (*document.Document, error) {
	opp, err := s.deps.Opportunities.Get(ctx, oppID)
	if err != nil {
		return nil, err
	}
	if !atGate(opp.ProposalStatus) {
		return nil, fmt.Errorf("draft for %s is not at the review gate (status %q)", oppID, opp.ProposalStatus)
	}
	return s.deps.Documents.UpdateSection(oppID, sectionID, body, "human")
}

// Approve is the gate's go decision: the human is done editing, and the
// real Final Review agent runs on the document exactly as the human left
// it. The verdict lands back on the opportunity status and, for issues, as
// document flags.
func (s *Service) Approve(ctx context.Context, oppID string) error {
	opp, err := s.deps.Opportunities.Get(ctx, oppID)
	if err != nil {
		return err
	}
	if !atGate(opp.ProposalStatus) {
		return fmt.Errorf("proposal %s is not at the review gate (status %q)", oppID, opp.ProposalStatus)
	}
	if !s.claim(oppID) {
		return fmt.Errorf("proposal %s already has a stage running", oppID)
	}
	if err := s.setStatus(ctx, oppID, StatusReviewRunning); err != nil {
		s.release(oppID)
		return err
	}
	s.spawn(oppID, s.runFinalReview)
	return nil
}

// RequestChanges is the gate's other decision: the draft goes back to the
// Writer with the human's note recorded in the document history.
func (s *Service) RequestChanges(ctx context.Context, oppID, note string) error {
	opp, err := s.deps.Opportunities.Get(ctx, oppID)
	if err != nil {
		return err
	}
	if !atGate(opp.ProposalStatus) {
		return fmt.Errorf("proposal %s is not at the review gate (status %q)", oppID, opp.ProposalStatus)
	}
	if !s.claim(oppID) {
		return fmt.Errorf("proposal %s already has a stage running", oppID)
	}
	if note = strings.TrimSpace(note); note != "" {
		if _, err := s.deps.Documents.AppendRevisionNote(oppID, "human", "Request changes: "+note); err != nil {
			s.release(oppID)
			return err
		}
	}
	if err := s.setStatus(ctx, oppID, StatusWriterRunning); err != nil {
		s.release(oppID)
		return err
	}
	s.spawn(oppID, s.runRevision)
	return nil
}

// Submit is always a human act: it marks the proposal submitted. Kaimi
// never submits on its own.
func (s *Service) Submit(ctx context.Context, oppID string) error {
	opp, err := s.deps.Opportunities.Get(ctx, oppID)
	if err != nil {
		return err
	}
	if opp.ProposalStatus != StatusReadyToSubmit {
		return fmt.Errorf("proposal %s is not ready to submit (status %q)", oppID, opp.ProposalStatus)
	}
	return s.setStatus(ctx, oppID, StatusSubmitted)
}

// atGate reports whether the proposal is paused for the human.
func atGate(status string) bool {
	return status == StatusGate || status == StatusReviewNeedsHuman
}

// claim/release guard against concurrent stages on one opportunity.
func (s *Service) claim(oppID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running[oppID] {
		return false
	}
	s.running[oppID] = true
	return true
}

func (s *Service) release(oppID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.running, oppID)
}

// spawn runs one background stage with its own context: the HTTP request
// that triggered it ends immediately, while the agents keep working and
// persist status at every transition (the UI polls via auto-refresh).
func (s *Service) spawn(oppID string, stage func(ctx context.Context, oppID string)) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.release(oppID)
		ctx, cancel := context.WithTimeout(context.Background(), stageTimeout)
		defer cancel()
		stage(ctx, oppID)
	}()
}

// runDraftPipeline executes Outline then Writer, building the proposal
// document, and pauses at the human gate.
func (s *Service) runDraftPipeline(ctx context.Context, oppID string) {
	opp, err := s.deps.Opportunities.Get(ctx, oppID)
	if err != nil {
		return
	}

	out, res, err := s.deps.Outline.Run(ctx, opp)
	if err != nil || res == nil || res.IsFailed() || out == nil {
		s.failStatus(ctx, oppID, "outline:failed")
		return
	}

	doc := &document.Document{
		OpportunityID: opp.ID,
		Title:         out.Title,
		Sections:      sectionsFromOutline(out),
	}
	if err := s.deps.Documents.Create(doc, "outline",
		fmt.Sprintf("Outline skeleton: %d sections", len(out.Sections))); err != nil {
		s.failStatus(ctx, oppID, "outline:failed")
		return
	}

	if err := s.setStatus(ctx, oppID, StatusWriterRunning); err != nil {
		return
	}
	s.draftSections(ctx, oppID, opp, out, "Draft from the technical writer")
}

// runRevision re-runs the Writer over the existing document after a
// change request, then returns to the gate.
func (s *Service) runRevision(ctx context.Context, oppID string) {
	opp, err := s.deps.Opportunities.Get(ctx, oppID)
	if err != nil {
		return
	}
	doc, err := s.deps.Documents.Get(oppID)
	if err != nil {
		s.failStatus(ctx, oppID, "writer:failed")
		return
	}
	s.draftSections(ctx, oppID, opp, outlineFromDocument(doc), "Revised draft after change request")
}

// draftSections runs the Writer agent, applies its draft to the document
// section by section, and pauses at the gate.
func (s *Service) draftSections(ctx context.Context, oppID string, opp *opportunity.Opportunity, out *outline.Outline, note string) {
	draft, res, err := s.deps.Writer.Run(ctx, writer.Input{
		Opportunity: opp,
		Outline:     out,
		Profile:     s.deps.Profile,
	})
	if err != nil || res == nil || res.IsFailed() {
		s.failStatus(ctx, oppID, "writer:failed")
		return
	}
	bodies := splitDraft(draft, out)
	if len(bodies) > 0 {
		if _, err := s.deps.Documents.ReplaceSections(oppID, bodies, "writer", note); err != nil {
			s.failStatus(ctx, oppID, "writer:failed")
			return
		}
	}
	_ = s.setStatus(ctx, oppID, StatusGate)
}

// runFinalReview renders the document as the human left it and runs the
// real Final Review agent on that revision.
func (s *Service) runFinalReview(ctx context.Context, oppID string) {
	opp, err := s.deps.Opportunities.Get(ctx, oppID)
	if err != nil {
		return
	}
	doc, err := s.deps.Documents.Get(oppID)
	if err != nil {
		s.failStatus(ctx, oppID, "final-review:failed")
		return
	}

	res, err := s.deps.Review.Review(ctx, finalreview.Input{
		Draft:       doc.Markdown(),
		Opportunity: opp,
		Outline:     outlineFromDocument(doc),
	})
	if err != nil || res == nil {
		s.failStatus(ctx, oppID, "final-review:failed")
		return
	}

	switch {
	case res.Status == "ready_to_submit":
		// Resolve any outstanding flags: the final pass is clean.
		if len(doc.Flags) > 0 {
			flags := make([]document.Flag, len(doc.Flags))
			copy(flags, doc.Flags)
			for i := range flags {
				flags[i].Resolved = true
			}
			_, _ = s.deps.Documents.SetFlags(oppID, flags, "final-review", "Final pass clean — flags resolved")
		}
		_ = s.setStatus(ctx, oppID, StatusReadyToSubmit)
	case res.NeedsHuman():
		flags := flagsFromResult(res.Flags)
		if len(flags) > 0 {
			_, _ = s.deps.Documents.SetFlags(oppID, flags, "final-review", "Final review issues")
		}
		_ = s.setStatus(ctx, oppID, StatusReviewNeedsHuman)
	default:
		s.failStatus(ctx, oppID, "final-review:failed")
	}
}

// setStatus persists a ProposalStatus transition.
func (s *Service) setStatus(ctx context.Context, oppID, status string) error {
	opp, err := s.deps.Opportunities.Get(ctx, oppID)
	if err != nil {
		return err
	}
	opp.ProposalStatus = status
	opp.UpdatedAt = s.Now()
	return s.deps.Opportunities.Save(ctx, opp)
}

// failStatus persists a failure status; the error itself was already
// recorded by the agent's Result and the stage halts here.
func (s *Service) failStatus(ctx context.Context, oppID, status string) {
	_ = s.setStatus(ctx, oppID, status)
}

// sectionsFromOutline converts the Outline agent's sections into document
// sections, preserving order.
func sectionsFromOutline(out *outline.Outline) []document.Section {
	secs := make([]document.Section, 0, len(out.Sections))
	for _, s := range out.Sections {
		secs = append(secs, document.Section{
			ID:      s.ID,
			Heading: s.Title,
			Status:  "outlined",
		})
	}
	return secs
}

// outlineFromDocument rebuilds a minimal Outline from the document so the
// Writer (revisions) and Final Review (section checks) can run against the
// document as it stands. Formatting rules are unspecified — agents must
// not invent defaults for them.
func outlineFromDocument(doc *document.Document) *outline.Outline {
	sections := make([]outline.Section, 0, len(doc.Sections))
	for _, s := range doc.Sections {
		sections = append(sections, outline.Section{
			ID:       s.ID,
			Title:    s.Heading,
			Required: true,
		})
	}
	return &outline.Outline{
		OpportunityID: doc.OpportunityID,
		Title:         doc.Title,
		Sections:      sections,
		FormattingRules: &outline.FormattingRules{
			PageLimit:   &outline.FormattingRule{},
			Font:        &outline.FormattingRule{},
			Margins:     &outline.FormattingRule{},
			LineSpacing: &outline.FormattingRule{},
			FileFormat:  &outline.FormattingRule{},
		},
	}
}

// splitDraft maps the Writer's draft text ("\n## Title\n body" per section)
// back onto document section ids by heading.
func splitDraft(draft string, out *outline.Outline) map[string]string {
	byTitle := make(map[string]string, len(out.Sections))
	for _, s := range out.Sections {
		byTitle[s.Title] = s.ID
	}
	bodies := make(map[string]string)
	chunks := strings.Split(draft, "\n## ")
	for _, chunk := range chunks {
		title, body, found := strings.Cut(chunk, "\n")
		if !found {
			continue
		}
		id, ok := byTitle[strings.TrimSpace(title)]
		if !ok {
			continue
		}
		if body = strings.TrimSpace(body); body != "" {
			bodies[id] = body
		}
	}
	return bodies
}

// flagsFromResult converts the Final Review agent's issue flags
// ("issue_1", "issue_2", …) into document flags.
func flagsFromResult(resultFlags map[string]string) []document.Flag {
	var flags []document.Flag
	for i := 1; ; i++ {
		detail, ok := resultFlags[fmt.Sprintf("issue_%d", i)]
		if !ok {
			break
		}
		flags = append(flags, document.Flag{
			Title:  detail,
			Detail: "Flagged by the final review agent",
		})
	}
	return flags
}
