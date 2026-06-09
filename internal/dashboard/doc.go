// Package dashboard provides the store-backed view/query helper for the Kaimi
// opportunity-pipeline dashboard (GitHub issue #108, approved for Phase 0).
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
package dashboard
