// Command desktop is the Kaimi desktop dashboard: a Wails v2 (Go) application
// that wraps the shared Kaimi packages in a native OS window (Windows + macOS).
//
// Why this lives here: per ADR-001 (docs/desktop/adr-001-stack.md) the desktop
// client reuses internal/store, internal/opportunity, and internal/dashboard
// directly rather than reimplementing them. The window is intentionally thin —
// all testable, platform-independent logic lives in internal/desktop, which has
// no GUI dependency. This file's package (main) only wires that backend into a
// Wails window and binds it to the webview.
//
// Build constraints: the Wails entrypoint (main.go) compiles only on Windows
// and macOS, where a webview runtime exists. A stub (main_unsupported.go) keeps
// the package compiling on other platforms (e.g. the Linux CI runner) so
// `go test ./...` stays green without a Wails/CGO toolchain.
//
// Scope (issue #138): boot a window and list opportunities from the local store
// with derived pipeline stage. Design parity, onboarding, the offline editor,
// and cloud sync are later tickets.
//
// Usage:
//
//	desktop [-store <path>]
//
// The store path resolves via -store, then $KAIMI_STORE_PATH, then a per-user
// default (see internal/desktop.ResolveStorePath).
package main
