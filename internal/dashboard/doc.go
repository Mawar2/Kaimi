// Package dashboard provides the HTTP server and supporting logic for the
// Kaimi pipeline dashboard. It reads opportunity data through the store.Store
// interface and presents it as a read-only web UI.
//
// Wave 1 scope: stage derivation, per-stage counts, and HTTP server skeleton.
// Wave 2 scope: list and detail handlers.
// Wave 3 scope: HTML templates.
// Wave 4 scope: human-action endpoints (select, approve, reject).
package dashboard
