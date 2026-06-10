# Kaimi Desktop — build & run

**Last updated:** 2026-06-09

The desktop dashboard (`cmd/desktop`) is a **Wails v2 (Go)** app — see
[ADR-001](./adr-001-stack.md) for why. This ticket (#138) delivers the scaffold:
a window that reads a local Kaimi store and lists opportunities with their
derived pipeline stage. Design parity (#139), the offline editor, onboarding,
and cloud sync are later tickets.

## Layout

| Path | What it is |
|---|---|
| `internal/desktop/` | UI-agnostic backend (store read + list view-model). **No GUI dependency** — fully unit-tested, compiles everywhere (including Linux CI). |
| `cmd/desktop/main.go` | Wails entrypoint. Build-constrained to `windows || darwin`. |
| `cmd/desktop/main_unsupported.go` | Stub for other platforms so `go build/test ./...` stays green on the Linux CI runner without a Wails/CGO toolchain. |
| `cmd/desktop/frontend/dist/` | Minimal embedded webview UI (HTML/JS). Bundled into the binary → offline, no network fetch. |
| `cmd/desktop/wails.json` | Wails project config (Windows + `darwin/universal` targets). |
| `cmd/desktop/build/` | **Generated** by `wails build` (icon, manifest, binary). Git-ignored; regenerated on demand. Branded assets land in #139. |

## Prerequisites

- **Go 1.25+**
- **Node.js + npm** (Wails uses them for frontend tooling; this scaffold ships a
  prebuilt `frontend/dist`, so no `npm install` is needed yet)
- **Wails v2 CLI:** `go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0`
  (the module is pinned to `v2.12.0` in `go.mod`)
- **Windows:** the **WebView2 runtime** (ships with Windows 11; evergreen on
  Windows 10). `wails doctor` confirms it.
- **macOS:** Xcode Command Line Tools.

Run `wails doctor` to verify your environment.

## Run (development)

From the project directory (hot reload):

```sh
cd cmd/desktop
wails dev                      # uses the default store path
wails dev -- -store C:\path\to\store   # point at a specific local store
```

You can also run the plain Go binary (no hot reload):

```sh
go run ./cmd/desktop -store C:\path\to\store
```

## Build (production)

```sh
cd cmd/desktop
wails build                              # current OS -> build/bin/kaimi-desktop[.exe]
wails build -platform darwin/universal   # macOS universal binary (run on macOS)
```

The output is a single binary in `cmd/desktop/build/bin/`.

## Choosing the store

The app reads an existing local Kaimi store directory (the same JSON layout the
pipeline writes — `<store>/queue/<id>.json`). Path resolution, highest priority
first:

1. `-store <path>` flag
2. `KAIMI_STORE_PATH` environment variable
3. Default: `<user-config-dir>/Kaimi/store` (e.g. `%AppData%\Kaimi\store` on Windows)

A missing or empty store is **not** an error — the app shows a calm empty state
and creates the directory. (Offline/empty states are slate, never amber; amber
is reserved for "a human is needed".)

## Tests, vet, lint

All run cross-platform because the testable logic lives in `internal/desktop`
(no Wails import):

```sh
go test ./...
go vet ./...
golangci-lint run
```

The Wails entrypoint compiles only on Windows/macOS; on Linux CI the stub keeps
`go build ./...` green.

## CI note (build job is optional this ticket)

`go test ./...` covers the backend on the standard Linux runner. A packaged
Windows/macOS `wails build` job (with the WebView2/Xcode toolchains) is a
later addition; it is not required for #138.
