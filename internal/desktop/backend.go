package desktop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Mawar2/Kaimi/internal/dashboard"
	"github.com/Mawar2/Kaimi/internal/store"
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
// shared internal/dashboard read-only Service over a local JSON store.
//
// Backend performs only reads; it never mutates the store (offline writes and
// the sync queue are later tickets — see ADR-001 and #140).
type Backend struct {
	svc       *dashboard.Service
	storePath string

	// Now supplies the current time for deadline computation. It is a field so
	// tests can inject a fixed clock; it defaults to time.Now.
	Now func() time.Time
}

// New opens (creating if absent) the local store at storePath and returns a
// Backend over it. A missing directory is created rather than treated as an
// error, so a first run on a fresh machine shows an empty state, not a crash.
func New(storePath string) (*Backend, error) {
	s, err := store.NewJSONStore(storePath)
	if err != nil {
		return nil, fmt.Errorf("open local store at %q: %w", storePath, err)
	}
	return &Backend{
		svc:       dashboard.NewService(s),
		storePath: storePath,
		Now:       time.Now,
	}, nil
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
