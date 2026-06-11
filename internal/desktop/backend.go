package desktop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mawar2/Kaimi/internal/dashboard"
	"github.com/Mawar2/Kaimi/internal/proposal"
	"github.com/Mawar2/Kaimi/internal/store"
	"github.com/Mawar2/Kaimi/internal/zone2view"
)

// storePathEnv is the environment variable that overrides the local store
// location. It lets a user point the desktop app at an existing synced store
// (e.g. a folder kept in sync with the cloud queue) without rebuilding.
const storePathEnv = "KAIMI_STORE_PATH"

// emptyStoreMessage is the calm, slate-toned empty state shown when the local
// store has no opportunities. Per the design system, offline/empty states are
// never amber — amber is reserved exclusively for "a human is needed".
const emptyStoreMessage = "No opportunities in your local store yet. " +
	"They sync from the nightly hunt when you're online."

// ResolveStorePath determines which local store directory the desktop app reads.
//
// Precedence, highest first:
//  1. override (a CLI flag value); used as-is when non-empty.
//  2. the KAIMI_STORE_PATH environment variable.
//  3. a sane per-user default: <UserConfigDir>/Kaimi/store.
//
// The path is not required to exist yet — New creates it on first use.
func ResolveStorePath(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if env := os.Getenv(storePathEnv); env != "" {
		return env, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve default store path: %w", err)
	}
	return filepath.Join(configDir, "Kaimi", "store"), nil
}

// Backend is the UI-agnostic data layer for the desktop dashboard. It wraps the
// shared internal/dashboard read-only Service for opportunity reads, and — when
// a proposal service is wired via WithProposals — the shared internal/proposal
// lifecycle for the Zone 2 flow (select, gate actions, drafting). Both the web
// dashboard and the desktop drive the same internal/proposal.Service, and both
// derive display state from internal/zone2view, so the two surfaces cannot
// disagree about a proposal's stage or criteria (issue #249).
type Backend struct {
	svc       *dashboard.Service
	proposals *proposal.Service // nil = read-only (no Zone 2 actions)
	storePath string

	// Now supplies the current time for deadline computation. It is a field so
	// tests can inject a fixed clock; it defaults to time.Now.
	Now func() time.Time
}

// Option configures optional Backend capabilities.
type Option func(*Backend)

// WithProposals enables the Zone 2 surfaces (select, proposals list, workspace,
// gate actions) backed by the shared proposal lifecycle service. The service
// must read/write the same store directory as the Backend.
func WithProposals(p *proposal.Service) Option {
	return func(b *Backend) { b.proposals = p }
}

// New opens (creating if absent) the local store at storePath and returns a
// Backend over it. A missing directory is created rather than treated as an
// error, so a first run on a fresh machine shows an empty state, not a crash.
func New(storePath string, opts ...Option) (*Backend, error) {
	s, err := store.NewJSONStore(storePath)
	if err != nil {
		return nil, fmt.Errorf("open local store at %q: %w", storePath, err)
	}
	b := &Backend{
		svc:       dashboard.NewService(s),
		storePath: storePath,
		Now:       time.Now,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b, nil
}

// now returns the backend's clock, defaulting to time.Now.
func (b *Backend) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

// StorePath returns the resolved local store directory the backend is reading.
func (b *Backend) StorePath() string { return b.storePath }

// ListResult is the view-model returned to the desktop UI for the list screen.
type ListResult struct {
	// StorePath is the local directory the rows were read from (shown in the UI
	// so the user knows which store is active).
	StorePath string `json:"storePath"`
	// Rows are the opportunity rows, each with a derived pipeline Stage.
	Rows []dashboard.OpportunityRow `json:"rows"`
	// Empty is true when there are no opportunities to show.
	Empty bool `json:"empty"`
	// Message is a friendly explanation to render when Empty is true.
	Message string `json:"message"`
}

// ListOpportunities reads the local store and returns the opportunity rows with
// their derived pipeline stage, sorted by the dashboard's default (deadline).
// When the store holds no opportunities it returns a friendly empty state
// instead of an error.
func (b *Backend) ListOpportunities(ctx context.Context) (ListResult, error) {
	now := time.Now
	if b.Now != nil {
		now = b.Now
	}

	rows, err := b.svc.List(ctx, dashboard.ListOptions{Now: now()})
	if err != nil {
		return ListResult{}, fmt.Errorf("list opportunities: %w", err)
	}

	res := ListResult{StorePath: b.storePath, Rows: rows}
	if len(rows) == 0 {
		res.Empty = true
		res.Message = emptyStoreMessage
	}
	return res, nil
}

// errProposalsDisabled is returned by the Zone 2 methods when no proposal
// service is wired (a read-only deployment).
var errProposalsDisabled = fmt.Errorf("proposal actions are not enabled on this backend")

// Select is the Zone 1 → Zone 2 bridge: the human chooses to pursue an
// opportunity. It starts the real draft pipeline (Outline → Writer) in the
// background, pausing at the human gate. It is a no-op error when proposals are
// not enabled.
func (b *Backend) Select(ctx context.Context, oppID string) error {
	if b.proposals == nil {
		return errProposalsDisabled
	}
	if err := b.proposals.Select(ctx, oppID); err != nil {
		return fmt.Errorf("select %s: %w", oppID, err)
	}
	return nil
}

// ProposalCard is the desktop view-model for one active-proposal card. Display
// state (StageIndex/State/When) comes from internal/zone2view — the same source
// the web uses — so the two surfaces agree (issue #246 B2).
type ProposalCard struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Agency     string    `json:"agency"`
	When       string    `json:"when"`       // named-teammate status phrase
	StageIndex int       `json:"stageIndex"` // 0-4 pipeline position
	State      string    `json:"state"`      // human|progress|done|submitted|failed
	Deadline   time.Time `json:"deadline"`   // zero when unset; the UI formats it
}

// ProposalsResult is the view-model for the desktop Proposals command view.
type ProposalsResult struct {
	StorePath string         `json:"storePath"`
	InFlight  int            `json:"inFlight"` // proposals the agents are working
	NeedsYou  int            `json:"needsYou"` // proposals paused at the human gate
	Cards     []ProposalCard `json:"cards"`
	Empty     bool           `json:"empty"`
}

// ListProposals returns every opportunity that has entered the Zone 2 pipeline
// (selected and beyond), as cards whose state is derived from the raw
// ProposalStatus via internal/zone2view — identical to the web command view, so
// the list and the workspace can never contradict each other.
func (b *Backend) ListProposals(ctx context.Context) (ProposalsResult, error) {
	rows, err := b.svc.List(ctx, dashboard.ListOptions{Now: b.now()})
	if err != nil {
		return ProposalsResult{}, fmt.Errorf("list proposals: %w", err)
	}
	res := ProposalsResult{StorePath: b.storePath}
	for i := range rows {
		row := &rows[i]
		// Skip opportunities still in triage (not yet pursued).
		if row.Stage == dashboard.StageHunted || row.Stage == dashboard.StageScored {
			continue
		}
		stageIndex, state := zone2view.View(row.ProposalStatus)
		res.Cards = append(res.Cards, ProposalCard{
			ID:         row.ID,
			Title:      row.Title,
			Agency:     row.Agency,
			When:       zone2view.StatusPhrase(stageIndex, state),
			StageIndex: stageIndex,
			State:      state,
			Deadline:   row.ResponseDeadline,
		})
		switch state {
		case "human":
			res.NeedsYou++
			res.InFlight++
		case "submitted":
			// terminal — not in flight
		default:
			res.InFlight++
		}
	}
	res.Empty = len(res.Cards) == 0
	return res, nil
}

// WorkspaceSection is one editable draft section in the workspace view-model.
type WorkspaceSection struct {
	ID      string `json:"id"`
	Heading string `json:"heading"`
	Body    string `json:"body"`
	Status  string `json:"status"`
}

// WorkspaceFlag is one open issue flagged on the draft (e.g. by Final Review).
type WorkspaceFlag struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// WorkspaceResult is the single-proposal workspace view-model. Stage/state and
// criteria are derived from internal/zone2view — the same source the web uses —
// so the desktop and web workspaces cannot disagree (issues #246 B2/B6, #249).
type WorkspaceResult struct {
	ID         string                `json:"id"`
	Title      string                `json:"title"`
	Agency     string                `json:"agency"`
	ScorePct   int                   `json:"scorePct"`
	Deadline   time.Time             `json:"deadline"`
	StageIndex int                   `json:"stageIndex"`
	State      string                `json:"state"`
	Phrase     string                `json:"phrase"`
	AtGate     bool                  `json:"atGate"`
	Version    int                   `json:"version"`
	LastEditor string                `json:"lastEditor"` // raw actor: "human"/"writer"/...; UI maps to a name
	HasDraft   bool                  `json:"hasDraft"`
	Sections   []WorkspaceSection    `json:"sections"`
	Criteria   []zone2view.Criterion `json:"criteria"`
	OpenFlags  []WorkspaceFlag       `json:"openFlags"`
}

// Workspace returns the single-proposal view-model for a pursued opportunity:
// gate state, the working draft's sections, the criteria grid (derived via the
// shared matcher), and any open flags. It errors for an unknown or not-yet-
// selected opportunity, or when proposals are disabled.
func (b *Backend) Workspace(ctx context.Context, oppID string) (WorkspaceResult, error) {
	if b.proposals == nil {
		return WorkspaceResult{}, errProposalsDisabled
	}
	opp, err := b.svc.Get(ctx, oppID)
	if err != nil {
		return WorkspaceResult{}, fmt.Errorf("workspace %s: %w", oppID, err)
	}
	if !opp.Selected {
		return WorkspaceResult{}, fmt.Errorf("opportunity %s is not in your proposals", oppID)
	}

	stageIndex, state := zone2view.View(opp.ProposalStatus)
	res := WorkspaceResult{
		ID:         opp.ID,
		Title:      opp.Title,
		Agency:     opp.Agency,
		Deadline:   opp.ResponseDeadline,
		StageIndex: stageIndex,
		State:      state,
		Phrase:     zone2view.StatusPhrase(stageIndex, state),
		AtGate:     state == "human",
	}
	if opp.Score > 0 {
		res.ScorePct = int(0.5 + opp.Score*100)
	}

	if doc, err := b.proposals.Document(oppID); err == nil {
		res.HasDraft = true
		res.Version = doc.Version
		if n := len(doc.Revisions); n > 0 {
			res.LastEditor = doc.Revisions[n-1].Actor
		}
		for _, s := range doc.Sections {
			res.Sections = append(res.Sections, WorkspaceSection{
				ID: s.ID, Heading: s.Heading, Body: s.Body, Status: s.Status,
			})
		}
		res.Criteria = zone2view.DeriveCriteria(opp.Requirements, strings.ToLower(doc.Markdown()), doc.OpenFlagTexts())
		for _, f := range doc.Flags {
			if !f.Resolved {
				res.OpenFlags = append(res.OpenFlags, WorkspaceFlag{Title: f.Title, Detail: f.Detail})
			}
		}
	}
	return res, nil
}

// UpdateSection records a human edit to one draft section. Edits are only
// meaningful while the proposal is paused at the gate; the service enforces that.
func (b *Backend) UpdateSection(ctx context.Context, oppID, sectionID, body string) error {
	if b.proposals == nil {
		return errProposalsDisabled
	}
	if _, err := b.proposals.UpdateSection(ctx, oppID, sectionID, body); err != nil {
		return fmt.Errorf("update section %s/%s: %w", oppID, sectionID, err)
	}
	return nil
}

// Approve is the gate's go decision: the real Final Review runs on the document
// as the human left it, then the proposal becomes ready to submit (or returns to
// the gate with flags).
func (b *Backend) Approve(ctx context.Context, oppID string) error {
	if b.proposals == nil {
		return errProposalsDisabled
	}
	if err := b.proposals.Approve(ctx, oppID); err != nil {
		return fmt.Errorf("approve %s: %w", oppID, err)
	}
	return nil
}

// RequestChanges is the gate's other decision: the draft goes back to the Writer
// with the human's note recorded in the document history.
func (b *Backend) RequestChanges(ctx context.Context, oppID, note string) error {
	if b.proposals == nil {
		return errProposalsDisabled
	}
	if err := b.proposals.RequestChanges(ctx, oppID, note); err != nil {
		return fmt.Errorf("request changes %s: %w", oppID, err)
	}
	return nil
}

// Submit is always a human act: it marks a ready proposal submitted. Kaimi never
// submits on its own.
func (b *Backend) Submit(ctx context.Context, oppID string) error {
	if b.proposals == nil {
		return errProposalsDisabled
	}
	if err := b.proposals.Submit(ctx, oppID); err != nil {
		return fmt.Errorf("submit %s: %w", oppID, err)
	}
	return nil
}

// DraftMarkdown returns the proposal's working draft as Markdown — the desktop's
// real, openable draft artifact (issue #246 B3 parity), not a dead label.
func (b *Backend) DraftMarkdown(oppID string) (string, error) {
	if b.proposals == nil {
		return "", errProposalsDisabled
	}
	doc, err := b.proposals.Document(oppID)
	if err != nil {
		return "", fmt.Errorf("draft for %s: %w", oppID, err)
	}
	return doc.Markdown(), nil
}
