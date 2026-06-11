# Goal: Wire the desktop to the real backend, at parity with the web QA fixes

**Owner loop.** Branch `feature/desktop-live-backend` off `feature/desktop-submitted-archive`.
Worktree `C:\Users\Owner\Kaimi-desktop-live`. Decision by Malik 2026-06-11: replace the desktop's
mock data with live data so it runs the **same fixed Zone-2 logic as the web** (true parity), not
a cosmetic prototype patch. **Human merges** (CLAUDE.md). This is desktop-epic-scale — go phase by
phase, surface the architectural forks, don't guess.

## Where things stand (verified 2026-06-11)
- Desktop = Wails app. Backend `internal/desktop/backend.go` already wraps `dashboard.Service`
  (read-only) and binds only `ListOpportunities`. Frontend `cmd/desktop/frontend` = React/Vite
  prototype; `api.js` calls the real backend ONLY for the opps list, everything else
  (proposals/workspace/gate/criteria/submitted) is **mock data in `data.js`** + a client-side
  simulated state machine in `App.jsx` (`toggleNet`, timers).
- `internal/proposal.Service` is the SHARED Zone-2 backend ("both the web dashboard and the desktop
  app call this service" — epic #153). The web (`internal/dashboard`) wraps it; the desktop does not yet.
- The QA fixes (B1,B2,B3,B4,B6) live in `internal/dashboard` on **PR #247** (`fix/dashboard-zone2-qa`,
  off main). This desktop branch does NOT have them (fillShellCounts hits: 0). Parity REQUIRES them.

## The QA bugs to reach parity on (from the web work, #246/#247)
- B1 sidebar counts shown on every screen incl. workspace.
- B2 list & workspace derive state from ONE source (raw ProposalStatus), never contradict.
- B3 the working draft is a real artifact (download/open), not a dead label.
- B4 gate actions (Approve / Request changes) give visible confirmation.
- B6 criteria use term-overlap matching + honest "could not auto-confirm", no false "missing".

## Plan (phased; PAUSE at the two ★ forks for Malik before heavy build)

### Phase 0 — Baseline (do first)
- Build the frontend (`cd cmd/desktop/frontend && npm ci && npm run build`) so `dist` exists and
  `go build ./cmd/desktop` works on Windows. Run the app (or the vite dev frontend) and QA the mock
  prototype to reproduce the exact bugs Malik saw (nav counts, state, criteria, gate feedback).
  Record a baseline (screenshots, which bugs reproduce, which are mock-only).

### Phase 1 ★ — Integration architecture (PROPOSE, then PAUSE for sign-off)
- Bring PR #247's `internal/dashboard` fixes onto this branch (merge `fix/dashboard-zone2-qa` or
  cherry-pick), resolving conflicts. This is the source of B1/B2/B3/B4/B6 logic.
- Decide how the desktop backend reuses the fixed Zone-2 view derivation (stage/state/criteria).
  Options to evaluate: (a) export the needed helpers from `internal/dashboard`; (b) extract a shared
  `internal/zone2view` package both web+desktop use; (c) have `internal/desktop` wrap
  `internal/proposal.Service` directly + a thin exported view layer. Recommend the least-duplication
  option that keeps ONE source of truth for B2/B6. **Surface the choice + the #247/desktop-archive
  merge-ordering risk to Malik before building.**

### Phase 2 — Desktop backend over the live service (TDD, internal/desktop/backend_test.go)
- Expand `internal/desktop.Backend` to wrap `internal/proposal.Service` and expose GUI-free methods
  + JSON-friendly result types for: Select(oppID), ListProposals(), GetWorkspace(oppID)
  (stage, state, phrase, sections, criteria, openFlags, versionLabel, atGate, counts), UpdateSection,
  Approve, RequestChanges(note), Submit, DraftMarkdown(oppID) (B3), Submitted archive.
- Reuse the shared view derivation from Phase 1 so B2/B6 come for free. TDD each method.

### Phase 3 — Wails bindings (cmd/desktop/main.go)
- Bind the new App methods; keep `App` thin (delegates to backend), matching the existing pattern.
- Regenerate wailsjs bindings if needed.

### Phase 4 — Frontend: mock → live
- Rewire `api.js` + `proposals.jsx`/`workspace.jsx`/`editor.jsx`/`submitted.jsx`/`App.jsx` to the
  bound methods; remove the `data.js` mock paths (keep a browser-only fallback if cheap). Apply the
  UI behaviors: B1 counts on every screen, B4 confirmation feedback, B3 draft download/open, and let
  B2/B6 fall out of the live backend. Honest criteria copy.

### Phase 5 — Build, run, QA-verify (gstack-browse on the vite frontend or the Wails webview)
- Rebuild dist, run, drive the full flow: opps → Select → workspace (counts, draft fills, gate),
  Request changes (feedback + revise), Approve (Vera → ready), Submit → Submitted archive. Confirm
  list/workspace state agree and criteria never false-flags. Screenshot before/after per bug.

## Discipline
- TDD for all Go (backend_test.go); frontend verified via QA (browser). `go build ./... && go test ./...`
  + golangci-lint green before any PR. Atomic commits `NNN_description`. Agents never merge.
- Note merge-ordering: this branch depends on #247; flag conflicts/sequencing for the human.

## PROGRESS (loop updates every iteration)
- [x] Tracking issue created with AC — Mawar2/Kaimi#249
- [x] Phase 0 baseline — frontend builds (npm ci + build → dist), `go build ./...` OK, desktop runs (vite :5173). Findings: B1 does NOT repro (SPA shares sidebar state, counts show on workspace); desktop artifacts are mock docx/xlsx (not draft.md/document.json); criteria/flags are mock text. Confirms: real fix = wire to live backend, not patch mock JSX. Screenshots DT-00..DT-03.
- [x] Phase 1 ★ — Malik signed off; #247 merged clean; `internal/zone2view` extracted (View/StatusPhrase/StageNames/RequirementAddressed), dashboard delegates. zone2view + dashboard + desktop + proposal tests green, build/gofmt/vet clean (commits 854ff6a, 7990152).
- [x] Phase 2 desktop backend over live service — Select, ListProposals, Workspace, UpdateSection, Approve, RequestChanges, Submit, DraftMarkdown; web+desktop share zone2view (View/StatusPhrase/DeriveCriteria) so B2/B6 are single-source. TDD throughout; desktop+dashboard+zone2view green, build/gofmt/vet clean. commits 6aac54e, 9d4902e, 94419ef.
- [x] Phase 3 Wails bindings — App wrappers for Select/ListProposals/Workspace/UpdateSection/Approve/RequestChanges/Submit/DraftMarkdown; main() wires internal/proposal.Service over the store (newProposalService, stub default + -live-writer/-live-review). cmd/desktop builds clean (Windows), gofmt/vet clean. commit 3758f28. NOTE: frontend calls window.go.main.App.* directly (no wailsjs regen needed). ⚠ QA REALITY: live bindings only work in the Wails webview, NOT a plain browser — vite dev falls back to mock. So Phase 5 browser QA verifies the mock render + no-regressions; live flow verified via backend unit tests + structural parity (shared zone2view/proposal.Service) + optional manual Wails smoke test (surface to Malik at Phase 5).
- [~] Phase 4 frontend rewire — api.js seam (e30e505) + core component rewire DONE (6181f54): App.jsx loads proposals via getProposals + reloads after live actions (pursue/approve/requestChanges/submit), B4 toast, offline-sim guarded to mock-only, workspace via getWorkspace; api.js getWorkspace normalized (criteria from zone2view = B6); workspace.jsx ReviewCard consumes live criteria/flags/summary + B3 Download draft.md (draftMarkdown→file). B1 counts inherited (sidebar from live proposals), B2 inherited. vite build clean; browser mock-fallback renders the gate w/ download button + no console errors (DT-04). All 5 QA fixes live-wired in the core flow. REMAINING (minor): editor.jsx (wire updateSection for section edits) + submitted.jsx archive still mock — follow-ups, not blocking QA-fix parity.
- [ ] Phase 5 QA-verified end-to-end (all 5 bug behaviors correct on desktop)
- [ ] build + go test + golangci-lint green
- [ ] PR opened for Malik to merge (references the issue + #246)

### Iteration log
(append: date — what changed — commit — verify result)
- 2026-06-11 — Phase 4b: rewired App.jsx + workspace.jsx + api.js(getWorkspace) from mock to live bindings with browser mock fallback. Live: proposals load/reload, gate actions call backend + B4 toast, offline-sim guarded mock-only, workspace view-model + criteria via zone2view (B6), B3 Download draft.md. vite build clean; browser mock render verified (gate + new download button, no console errors, screenshot DT-04-workspace-rewired.png). commit 6181f54. All 5 QA fixes live in the core flow. NEXT: minor — wire editor.jsx (updateSection) + submitted.jsx; then Phase 5 (offer Malik a Wails smoke test) + open PR.
- 2026-06-11 — Phase 4a: api.js seam — added live Wails wrappers (getProposals/getWorkspace/pursue/approveProposal/requestChanges/submitProposal/updateSection/draftMarkdown) over window.go.main.App.*, each with bundled-demo/no-op fallback so vite dev (no window.go) still renders. getProposals maps ProposalsResult→screen shape (state from zone2view). vite build clean; App.jsx untouched so browser render unchanged. commit e30e505. Surfaced the Wails-vs-browser verify gap; Malik chose PROCEED (he verifies live at PR). NEXT (Phase 4b): rewire components to consume the seam.
- 2026-06-11 — Phase 3: bound the 8 Zone-2 App methods in cmd/desktop/main.go (thin delegations) + newProposalService wiring the shared internal/proposal.Service over the local store (stub default; -live-writer/-live-review enable Gemini). cmd/desktop builds clean on Windows, gofmt/vet clean. commit 3758f28. Surfaced the Wails-vs-browser QA limitation. NEXT: Phase 4 — rewire frontend api.js + jsx from data.js mock to window.go.main.App.* (B1 counts everywhere, B3 draft download via DraftMarkdown, B4 gate-action feedback; B2/B6 inherited). Keep a browser-mode mock fallback so vite dev still renders.
- 2026-06-11 — Phase 2c: Backend.Workspace view-model (gate state/phrase via zone2view, sections, criteria, open flags, version+lastEditor, scorePct). Added zone2view.Criterion + DeriveCriteria (single-source B6); dashboard delegates, CritItem retired (WorkspaceData.Criteria is []zone2view.Criterion). TDD red→green (TestWorkspaceViewModel: paraphrase must-have reads met; TestWorkspaceRejectsUnselected; zone2view TestDeriveCriteria). desktop+dashboard+zone2view green, build/gofmt/vet clean. commit 94419ef. PHASE 2 COMPLETE. NEXT: Phase 3 — bind the new Backend methods in cmd/desktop/main.go (App wrappers) + regenerate wailsjs if needed.
- 2026-06-11 — Phase 2b-actions: desktop Backend gains UpdateSection/Approve/RequestChanges/Submit + DraftMarkdown (B3, real draft markdown). Thin delegations w/ read-only guards. TDD red→green (TestGateActionsFlow: select→gate→edit→approve→ready→submit, draft reflects edit; TestRequestChangesReturnsToGate; require-service covers all mutators). build ./... OK, desktop pkg green, gofmt/vet clean. commit 9d4902e. NEXT (Phase 2c): Workspace view-model (sections, criteria via shared zone2view.DeriveCriteria, open flags, version/lastEditor, atGate) — add zone2view.Criterion+DeriveCriteria and have the web delegate too (single-source B6). Then Phase 3 bindings.
- 2026-06-11 — Phase 2a: desktop Backend gains WithProposals option + Select + ListProposals over internal/proposal.Service; card state via zone2view (B2 parity). TDD red→green (TestSelectAndListProposals drives the real stub-agent pipeline to the gate; TestProposalActionsRequireService). Build ./... OK, desktop pkg green, gofmt/vet clean. commit 6aac54e. dist already gitignored on this branch. NEXT (Phase 2b): Workspace view-model (counts/sections/criteria/flags/atGate/draft via zone2view) + gate actions (Approve/RequestChanges/Submit/UpdateSection) + DraftMarkdown (B3).
- 2026-06-11 — Phase 1b: extracted internal/zone2view (pure View/StatusPhrase/StageNames/RequirementAddressed). TDD red→green (zone2view_test.go: View + StatusPhrase + 6-case criteria table). internal/dashboard delegates (stageNames alias, call sites → zone2view.*; removed duplicated funcs + criteria_test.go + unicode/proposal imports). build ./... OK, zone2view+dashboard+desktop+proposal tests green, gofmt/vet clean. commit 7990152. NEXT: Phase 2 — internal/desktop.Backend over internal/proposal.Service + zone2view (JSON view-models), TDD.
- 2026-06-11 — Phase 1a: Malik signed off (merge #247 now; shared logic = new internal/zone2view). Merged fix/dashboard-zone2-qa into feature/desktop-live-backend — CLEAN, no conflicts. `go build ./...` OK; internal/dashboard + internal/desktop + internal/proposal tests green. Next: extract internal/zone2view.
- 2026-06-11 — Phase 0: built frontend (npm ci, vite build → dist), `go build ./cmd/desktop` + `./...` now pass on Windows. Ran vite :5173, QA'd prototype (onboarding→opps→proposals→workspace gate). Baseline: desktop is mock below the opps list; B1 doesn't repro (SPA shared state), artifacts are mock docx/xlsx, criteria/flags mock. Build-fix needs no code change — dist must be built before `go build` (CI on linux skips desktop via build tag). Reached Phase 1 ★ — paused for Malik's architecture sign-off.
