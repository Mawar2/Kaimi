//go:build windows || darwin

package main

import (
	"context"
	"embed"
	"flag"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/Mawar2/Kaimi/internal/desktop"
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

func main() {
	storeFlag := flag.String("store", "", "path to the local Kaimi store directory (overrides $KAIMI_STORE_PATH)")
	flag.Parse()

	storePath, err := desktop.ResolveStorePath(*storeFlag)
	if err != nil {
		log.Fatalf("kaimi-desktop: resolve store path: %v", err)
	}
	backend, err := desktop.New(storePath)
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
