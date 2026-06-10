# ADR-001 — Desktop dashboard stack: Wails v2 (Go) over Tauri / Electron

**Status:** Proposed — awaiting Malik's approval on [issue #137](https://github.com/Mawar2/Kaimi/issues/137)
**Date:** 2026-06-10
**Deciders:** Malik (approver), implementing session (author)
**Supersedes:** none
**Related:** Epic [#136](https://github.com/Mawar2/Kaimi/issues/136); tickets [#138](https://github.com/Mawar2/Kaimi/issues/138) (scaffold, blocked by this), [#139](https://github.com/Mawar2/Kaimi/issues/139) (parity), [#140](https://github.com/Mawar2/Kaimi/issues/140) (sync, design-only); design handoff `DESKTOP.md`.

> ADR format: [Michael Nygard](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions) (Context · Decision · Consequences). Per ARCHITECTURE.md, ADRs are append-only. This file lives under `docs/desktop/` per the acceptance criteria of #137 (the ARCHITECTURE.md ADR pointer says `docs/adr/`; desktop ADRs are co-located with the desktop docs and cross-referenced from ARCHITECTURE.md).

---

## Context

Malik decided (2026-06-09) that Kaimi gets a **desktop dashboard** for Windows and macOS, usable **offline** while working on proposals, alongside the existing web dashboard. Epic #136 is the handoff brief; this ADR is the design-eagerly gate that #136 requires before any scaffold code: choosing the wrong shell here is expensive to unwind once views and build/signing pipelines are built on top of it.

This decision is **load-bearing because of what already exists in the repo**, not theoretical preferences. The facts that constrain the choice:

1. **The reuse surface is Go, and it is real.** The data layer and the dashboard's presentation logic are already implemented as importable Go packages:
   - `internal/store` — the `Store` interface (`Save`/`Get`/`List`/`Delete`) with a file-backed `NewJSONStore(basePath)` implementation. Offline-first falls out of this for free: the store *is* a local JSON directory.
   - `internal/opportunity` — the shared `Opportunity` schema (the single source of truth; must not be forked for desktop).
   - `internal/dashboard` — `DeriveStage` (deterministic pipeline-stage derivation), `NewService(store)` → `List`/`Get`/`CountsByStage` view-model builders, **and the Kaimi design system rendered in Go**: `StyleTag()` (tokens + `ui.css` embedded verbatim, issue #132), plus `StatusBadge`/`RecommendationPill`/`DeadlinePill`/`FitRing`/`MetaTag` components and the `brand.go` Kai-wave assets (issue #126).
   - The **web dashboard renders HTML server-side in Go** (`internal/dashboard/handler.go`, `cmd/dashboard/main.go`): `store.NewJSONStore(path)` → `dashboard.NewService(s)` → `dashboard.NewHandler(svc)`. There is no JavaScript frontend or build toolchain in the repo today.

2. **The team is Go-centric, and "legible Go" is a hard requirement.** ARCHITECTURE.md and CLAUDE.md state it twice: two people review and learn from this code, one newer to Go. A desktop backend in a *different* language (Node for Electron, Rust for Tauri) splits the team's mental model and the review surface, and means the desktop's data/store/stage logic cannot be the same code that Zone 1 and the web dashboard already use — it would be a second implementation to keep in sync with `internal/*`.

3. **Single-binary distribution is an established repo principle.** ARCHITECTURE.md lists single-binary deployment as a reason Go was chosen.

4. **The design handoff recommends "Electron or Tauri."** `DESKTOP.md` and the bundle README say to target Electron or Tauri. **That recommendation was made without knowledge of the Go reuse thesis** — the design team assumed a greenfield JavaScript/React frontend (the prototypes are React-via-Babel). Their actual architectural intent is narrower and is what matters: *"the same three-surface app wrapped in a desktop shell"* — i.e. wrap the existing UI in an OS webview with a native window, custom title bar, keychain, and offline behavior. **Wails satisfies that intent exactly**, using the same OS webview approach as Tauri, but with a Go backend instead of Rust. So this ADR does not contradict the design intent; it selects the shell that realizes it while honoring constraints (1)–(3) the design team didn't have.

5. **Offline-first is the product point**, but it is *client* offline, not pipeline offline. The nightly SAM.gov hunt and the live agent runs are server-side and stay online-only; the desktop lets a human read the synced queue, open proposals, **edit the working draft**, and make review-gate decisions offline, queuing those actions for replay on reconnect (DESKTOP.md). ARCHITECTURE.md currently lists "Offline/air-gapped operation" under *what the architecture does not do* — that bullet is about the **pipeline**, and must be reconciled (see the companion ARCHITECTURE.md update) so it is not read as forbidding this client.

---

## Decision

**Adopt Wails v2 (Go) as the desktop shell.** The desktop app is a Go binary that imports `internal/store`, `internal/opportunity`, and `internal/dashboard` directly, wrapped in a frameless OS-webview window (WebView2 on Windows, WKWebView on macOS) with a custom branded title bar.

Use the **current stable v2 line**, not v3 (still pre-release at time of writing — revisit in a later ADR when v3 is stable; nothing here precludes a v2→v3 migration).

### Frontend strategy (the consequential sub-decision)

Render the **existing Go design system into the webview** rather than introducing a parallel JavaScript frontend on day one:

- For the read-only parity views (#138 scaffold list, #139 overview / table / detail), the webview is served the **same Go-rendered HTML + `StyleTag()` design system** the web dashboard already produces. The React prototypes in the bundle are treated as the **visual spec**, exactly as the web dashboard treats them — not as code to port. This proves the reuse thesis end-to-end with zero duplicated tokens and no JS build toolchain.
- **Fonts:** the web dashboard intentionally stays on the system font stack because it must not ship external assets (ux-spec.md "Technology Constraints"). A desktop app is precisely the *"surface that may ship external assets in later phases"* that ux-spec.md anticipates: it **bundles Figtree + IBM Plex Mono locally** (embedded, not fetched from Google Fonts — fetching would break offline) to reach the design's full type fidelity. One token source, one design system, an additive font layer on desktop.
- **Defer a JS frontend until a ticket actually needs it.** The richer desktop-only surfaces in `DESKTOP.md` — the six-step onboarding flow, the section-structured offline draft editor, animated drawers — are *not in #138 or #139* (#139 explicitly excludes proposal editing / human-review actions; the editor is not yet ticketed). If/when those tickets land and server-rendered HTML is the wrong tool for that interactivity, introduce a Wails frontend (Vite + a small framework) **sharing `kaimi/tokens.css` (= the same values as `tokens.go`) as the single token source** — recorded in a follow-up ADR at that time. We do not build that ahead of need (provision lazily).

This keeps #138/#139 minimal and maximally reuse-driven while leaving a clean seam for the interactive offline editor later.

### Out of scope for this ADR (and this ticket)

Scaffold code, UI work, installers, code-signing/notarization setup, and the local↔GCS sync design (#140) are explicitly *not* decided or built here. This ADR only fixes the shell and the frontend strategy.

---

## Options considered

| Criterion (weight) | **Wails v2 (Go)** | Tauri (Rust) | Electron (Node) |
|---|---|---|---|
| **Reuse of `internal/store`, `internal/dashboard`, `internal/opportunity` (highest)** | ✅ Direct `import` — same code as Zone 1 & web dashboard; zero logic duplication | ❌ Rust backend cannot import Go; needs a Go sidecar process or a reimplementation | ❌ Node backend cannot import Go; needs a Go sidecar process or a reimplementation |
| **Team language / "legible Go" (high)** | ✅ Backend is Go, the language the team builds everything in | ❌ Adds Rust — new language for a two-person Go-learning team | ⚠️ Adds Node/JS main process — more familiar than Rust, still a second runtime |
| **Offline-first JSON store (high)** | ✅ `NewJSONStore(localPath)` read directly in-process | ⚠️ Via sidecar or a Rust re-read of the JSON | ⚠️ Via sidecar or a Node re-read of the JSON |
| **Shares the existing design system** | ✅ Renders `StyleTag()` + Go components verbatim; `tokens.css` available if a JS layer is later added | ⚠️ Would consume `tokens.css`; backend logic still not shared | ⚠️ Same as Tauri |
| **Single-binary distribution (repo principle)** | ✅ One Go binary, assets embedded | ✅ Small native binary (+ webview) | ❌ Ships a full Chromium (~100–150 MB) |
| **Windows + macOS webview** | ✅ WebView2 (Win) / WKWebView (mac) | ✅ WebView2 / WKWebView (same model) | ✅ Bundled Chromium (no OS webview dependency) |
| **Ecosystem maturity (medium)** | ⚠️ Smaller ecosystem than Electron; v2 stable, v3 pre-release | ⚠️ Maturing; large Rust ecosystem but smaller desktop-app community than Electron | ✅ Largest, most mature desktop ecosystem |
| **Distribution friction (signing/notarization)** | ⚠️ macOS Developer-ID signing + notarization required (Apple tax, framework-independent); Windows WebView2 runtime dependency | ⚠️ Same macOS tax; same WebView2 dependency | ⚠️ Same macOS tax; no webview runtime dependency, but largest download |

### Why not Tauri
Tauri is the closest technical analogue (OS webview, small binary) and is a fine choice for a JS-frontend team. But its backend is **Rust**, which the Go reuse thesis and the "legible Go" requirement rule out: it cannot import `internal/*`, so the desktop's store/stage/view logic would either be a Rust reimplementation (a second source of truth to keep in sync — the exact drift ARCHITECTURE.md warns about) or a Go sidecar process bolted onto a Rust shell (more moving parts than Wails, with none of the reuse benefit Wails gives natively).

### Why not Electron
Electron's ecosystem maturity is its real advantage, and Node is more approachable than Rust. But it still cannot reuse the Go packages, adds a second runtime/language to a Go-learning team, and ships a full Chromium per app — directly against the single-binary principle. Its one genuine edge (no OS-webview runtime dependency) does not outweigh losing the reuse thesis that motivates the whole desktop effort.

---

## Honest trade-offs of choosing Wails v2

- **Smaller ecosystem than Electron.** Fewer plugins, fewer Stack Overflow answers, fewer prebuilt solutions for auto-update/installers. Mitigation: our scope is narrow (a read/review client), and the heavy lifting is in Go packages we own.
- **WebView2 runtime dependency on Windows.** Wails relies on the Evergreen WebView2 runtime. It ships by default on Windows 11 (the dev/target machine) and on current Windows 10; for older machines a bootstrapper can be bundled. Document it in the build instructions (#138).
- **Webview rendering quirks.** WebView2 (Chromium-based) and WKWebView (Safari/WebKit) are different engines, so CSS/JS can diverge slightly across OSes — the same cross-engine caveat Tauri has. Because our parity views are server-rendered HTML with a constrained, already-shipping CSS design system, exposure is low; test both engines in #139's QA script.
- **macOS code-signing & notarization.** Distributing outside a dev machine requires an Apple Developer ID, signing, and notarization. This is an Apple platform tax independent of framework choice (Tauri and Electron pay it too). It is **not** a scaffold concern — flag it for a dedicated distribution ticket; #138 only needs a local Windows build with the macOS target *configured*.
- **v2 vs v3.** Wails v3 is in pre-release; choosing v2 means a future migration is possible. v2 is stable and actively maintained; we accept this deliberately and will revisit via a new ADR when v3 is stable.
- **Frontend ceiling for rich interactions.** Server-rendered HTML is the right tool for the read-only parity views but will likely be the wrong tool for the offline editor's section-structured, click-to-edit, autosaving surface. We accept that and have left an explicit seam: a Wails JS frontend sharing `tokens.css`, decided in a follow-up ADR when that ticket exists. Choosing Wails does **not** lock us out of a JS frontend — Wails' normal mode *is* a JS frontend; we are simply not adopting one before it's needed.

---

## Dependency justification (per CLAUDE.md dependency rule)

Adopting Wails v2 introduces the `github.com/wailsapp/wails/v2` Go module and its build CLI.

- **Why a dependency at all / why not stdlib:** there is no stdlib path to a native OS window hosting a webview with `-webkit-app-region` drag regions, frameless chrome, and native menus. The realistic options are all third-party desktop frameworks; this ADR selects among them.
- **Why this one:** it is the only mainstream option whose backend is Go, which is the entire reason it wins (direct reuse of `internal/*`, single Go binary, team language). It replaces *more* potential dependencies than it adds — no Node/Rust toolchain, no sidecar IPC layer, no second design-token/store implementation.
- **Pinning:** `go get github.com/wailsapp/wails/v2@<latest-stable-v2>` then `go mod tidy`, with the exact version recorded on #138 when scaffolding actually adds it (this ticket adds **no** code, so the dependency is not yet added to `go.mod`).
- **Transitive footprint:** Wails pulls webview bindings and a small set of build/runtime deps; the scaffold ticket will report the resulting `go.mod`/`go.sum` delta for review.
- **CONVENTIONS.md note:** the repo's CLAUDE.md references a CONVENTIONS.md that is **not present in the tree**. Per CLAUDE.md's "new pattern → update CONVENTIONS.md" rule, the desktop scaffold (#138) introduces a new top-level pattern (`cmd/desktop` + a Wails project layout); that ticket should either restore/extend CONVENTIONS.md or, until it exists, document the layout decision in `docs/desktop/`. Flagged here so it isn't silently skipped.

---

## Consequences

**Positive**
- The reuse thesis is proven structurally: the desktop imports the same store, schema, stage-derivation, view-models, and design system as the web dashboard. No fork of the `Opportunity` schema, no second design-token copy, no duplicated stage logic.
- Offline-first is nearly free: the local JSON store is the device source of truth; `internal/dashboard` is already read-only against the `Store` interface.
- One language for the whole system keeps the two-person team's review surface and mental model intact.
- A single Go binary per OS, consistent with ARCHITECTURE.md.

**Negative / accepted**
- We take on Wails' smaller ecosystem, the WebView2 runtime dependency, cross-engine webview testing, and (for real distribution) the macOS notarization tax.
- The offline draft editor will likely require a JS frontend later; we have scoped that out and left a clean seam rather than building it now.

**Follow-ups this unblocks / requires**
- ARCHITECTURE.md updated in the same change set: desktop client added to the component map with its phase; the offline-first-but-pipeline-online distinction recorded; the "Offline/air-gapped operation" not-supported bullet clarified; ADR-001 referenced.
- #138 may proceed **only after Malik approves this ADR on #137.** #138 adds the Wails dependency (with the pinned version on the ticket), `cmd/desktop` with a `doc.go`, boots a window on Windows, reads a local store via `internal/store`, and lists opportunities with `internal/dashboard.DeriveStage`.
- A future ADR will decide the JS-frontend question if/when the offline editor / onboarding tickets require it.
- #140 (local↔GCS sync) remains design-only and unaffected by this shell choice.
