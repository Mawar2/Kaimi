// Package desktop provides the UI-agnostic backend for the Kaimi desktop
// dashboard (cmd/desktop).
//
// It deliberately contains no Wails (or any GUI) dependency so that all of its
// logic is unit-testable on every platform — including the Linux CI runner,
// where the Wails entrypoint in cmd/desktop does not compile. The desktop
// window in cmd/desktop is a thin shell that binds the types defined here.
//
// The backend proves the desktop reuse thesis from ADR-001
// (docs/desktop/adr-001-stack.md): it reads an existing local Kaimi store
// through internal/store (no reimplementation, no schema fork) and shapes the
// list view through internal/dashboard (Service + DeriveStage) — the same
// packages the web dashboard runs on. Offline-first falls out of the
// file-backed JSON store: the local directory is the source of truth on device.
//
// Scope (issue #138): read-only listing with a friendly empty state. Proposal
// editing, the human-review gate, onboarding, and cloud sync are later tickets.
package desktop
