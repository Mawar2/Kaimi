// Package dashboard provides the HTTP server and supporting logic for the
// Kaimi pipeline dashboard. It reads opportunity data through the store.Store
// interface and presents it as a read-only web UI.
//
// It also provides the store-backed view/query helper for the Kaimi
// opportunity-pipeline dashboard (GitHub issue #108, approved for Phase 0).
//
// Wave 1 scope: stage derivation, per-stage counts, and HTTP server skeleton.
// Wave 2 scope: list and detail handlers.
// Wave 3 scope: HTML templates.
// Wave 4 scope: human-action endpoints (select, approve, reject).
//
// Phase 0 scope: this package adds read-only filter, sort, and deadline-flag
// helpers on top of the existing Store interface. It does not introduce any new
// agents, infrastructure, or stored fields — all stage derivation is
// deterministic from existing Opportunity fields.
//
// The package is read-only with respect to the Store: it only calls
// store.Store.Get and store.Store.List; it never calls store.Store.Save or
// store.Store.Delete. All mutation (opportunity selection, status updates) is
// performed by other packages and reflected here through the Store interface.
//
// Key types:
//   - Stage / DeriveStage: deterministic pipeline-stage derivation from Opportunity fields.
//   - Service / List / Get: view-model builder that loads opportunities from the Store,
//     applies filters, sorts, and computes derived display fields (DeadlineSoon, etc.).
//   - OpportunityRow: the view-model for a single row in the list view.
//   - Mark / FaviconLink / HeaderLockup: the Kai wave brand assets (GitHub issue
//     #126) consumed by the shared layout per docs/dashboard/ux-spec.md.
package dashboard
