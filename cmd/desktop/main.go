//go:build windows || darwin

package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/Mawar2/Kaimi/internal/desktop"
	"github.com/Mawar2/Kaimi/internal/document"
	"github.com/Mawar2/Kaimi/internal/finalreview"
	"github.com/Mawar2/Kaimi/internal/googledocs"
	"github.com/Mawar2/Kaimi/internal/outline"
	"github.com/Mawar2/Kaimi/internal/proposal"
	"github.com/Mawar2/Kaimi/internal/store"
	"github.com/Mawar2/Kaimi/internal/writer"
)

// assets holds the embedded webview frontend. Bundling assets keeps the app a
// single binary and fully offline — no network fetch for UI or fonts.
//
//go:embed all:frontend/dist
var assets embed.FS

// App is the Wails-bound application object. The webview calls its exported
// methods; it delegates all logic to the GUI-free internal/desktop.Backend.
type App struct {
	ctx     context.Context
	backend *desktop.Backend
}

// startup captures the Wails runtime context for later use.
func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// ListOpportunities is bound to the frontend. It returns the local store's
// opportunity rows (with derived pipeline stage) or a friendly empty state.
func (a *App) ListOpportunities() (desktop.ListResult, error) {
	return a.backend.ListOpportunities(a.ctx)
}

// The following methods are bound to the webview to drive the Zone 2 flow. Each
// is a thin delegation to the GUI-free Backend, which shares internal/proposal
// and internal/zone2view with the web so the two surfaces behave identically.

// Select pursues an opportunity, starting the draft pipeline to the human gate.
func (a *App) Select(id string) error { return a.backend.Select(a.ctx, id) }

// ListProposals returns the active-proposal cards for the command view.
func (a *App) ListProposals() (desktop.ProposalsResult, error) {
	return a.backend.ListProposals(a.ctx)
}

// Workspace returns the single-proposal view-model (gate state, sections,
// criteria, flags) for one pursued opportunity.
func (a *App) Workspace(id string) (desktop.WorkspaceResult, error) {
	return a.backend.Workspace(a.ctx, id)
}

// UpdateSection records a human edit to one draft section at the gate.
func (a *App) UpdateSection(id, sectionID, body string) error {
	return a.backend.UpdateSection(a.ctx, id, sectionID, body)
}

// Approve runs the final review on the human-edited draft.
func (a *App) Approve(id string) error { return a.backend.Approve(a.ctx, id) }

// RequestChanges sends the draft back to the writer with the human's note.
func (a *App) RequestChanges(id, note string) error {
	return a.backend.RequestChanges(a.ctx, id, note)
}

// Submit marks a ready proposal submitted (always a human act).
func (a *App) Submit(id string) error { return a.backend.Submit(a.ctx, id) }

// DraftMarkdown returns the proposal's working draft as Markdown for download.
func (a *App) DraftMarkdown(id string) (string, error) {
	return a.backend.DraftMarkdown(id)
}

func main() {
	storeFlag := flag.String("store", "", "path to the local Kaimi store directory (overrides $KAIMI_STORE_PATH)")
	liveWriter := flag.Bool("live-writer", false, "draft with the real Gemini writer (Vertex AI ADC; needs GCP_PROJECT_ID)")
	liveReview := flag.Bool("live-review", false, "run the Gemini compliance pass in Final Review (Vertex AI ADC; needs GCP_PROJECT_ID)")
	flag.Parse()

	storePath, err := desktop.ResolveStorePath(*storeFlag)
	if err != nil {
		log.Fatalf("kaimi-desktop: resolve store path: %v", err)
	}

	// Wire the shared Zone 2 lifecycle over the same local store, so Select and
	// the gate actions drive real agents. Defaults to offline stubs; the
	// -live-* flags enable Gemini (Vertex ADC).
	proposals, err := newProposalService(storePath, *liveWriter, *liveReview)
	if err != nil {
		log.Fatalf("kaimi-desktop: wire proposal service: %v", err)
	}
	backend, err := desktop.New(storePath, desktop.WithProposals(proposals))
	if err != nil {
		log.Fatalf("kaimi-desktop: open local store: %v", err)
	}
	log.Printf("kaimi-desktop: reading local store at %s", storePath)

	app := &App{backend: backend}

	if err := wails.Run(&options.App{
		Title:       "Kaimi Desktop",
		Width:       1100,
		Height:      800,
		MinWidth:    860,
		MinHeight:   600,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   app.startup,
		Bind:        []interface{}{app},
	}); err != nil {
		log.Fatalf("kaimi-desktop: %v", err)
	}
}

// newProposalService wires the shared Zone 2 lifecycle over the local store at
// storePath: the cached Outline docs client, and the Writer / Final Review
// agents. Both default to offline (stub Writer, deterministic Final Review) so
// the desktop runs with no credentials; -live-writer / -live-review swap in the
// real Gemini agents (Vertex AI ADC, needs GCP_PROJECT_ID). It mirrors
// cmd/dashboard so the desktop and web run the identical pipeline.
func newProposalService(storePath string, liveWriter, liveReview bool) (*proposal.Service, error) {
	opps, err := store.NewJSONStore(storePath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	docs, err := document.NewStore(storePath)
	if err != nil {
		return nil, fmt.Errorf("document store: %w", err)
	}
	docsClient, err := googledocs.NewClient(context.Background(), googledocs.Config{UseCached: true})
	if err != nil {
		return nil, fmt.Errorf("docs client: %w", err)
	}

	w := writer.New()
	if liveWriter {
		projectID := os.Getenv("GCP_PROJECT_ID")
		if projectID == "" {
			return nil, fmt.Errorf("-live-writer requires GCP_PROJECT_ID")
		}
		gen, err := writer.NewGeminiGenerator(context.Background(),
			projectID, envOr("GCP_REGION", "us-east4"), envOr("GEMINI_MODEL", "gemini-2.5-pro"))
		if err != nil {
			return nil, fmt.Errorf("gemini generator: %w", err)
		}
		w = writer.NewWithGenerator(gen)
		log.Printf("Technical Writer: LIVE Gemini drafting enabled (project %s)", projectID)
	} else {
		log.Printf("Technical Writer: stub mode (pass -live-writer for Gemini drafting)")
	}

	review := finalreview.New()
	if liveReview {
		projectID := os.Getenv("GCP_PROJECT_ID")
		if projectID == "" {
			return nil, fmt.Errorf("-live-review requires GCP_PROJECT_ID")
		}
		checker, err := finalreview.NewGeminiComplianceChecker(context.Background(),
			projectID, envOr("GCP_REGION", "us-east4"), envOr("GEMINI_MODEL", "gemini-2.5-pro"))
		if err != nil {
			return nil, fmt.Errorf("gemini compliance checker: %w", err)
		}
		review = finalreview.NewWithComplianceChecker(checker)
		log.Printf("Final Review: LIVE Gemini compliance pass enabled (project %s)", projectID)
	} else {
		log.Printf("Final Review: deterministic checks only (pass -live-review for Gemini compliance)")
	}

	return proposal.NewService(&proposal.Deps{
		Opportunities: opps,
		Documents:     docs,
		Outline:       outline.New(docsClient),
		Writer:        w,
		Review:        review,
	}), nil
}

// envOr returns the environment variable value or a fallback when unset.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
