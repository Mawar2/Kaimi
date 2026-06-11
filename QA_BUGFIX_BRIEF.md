# Goal: Fix Zone-2 dashboard QA bugs (web + desktop)

**Owner loop.** Branch `fix/dashboard-zone2-qa` off `main`. Worktree `C:\Users\Owner\Kaimi-qa-fix`.
Found during live QA on 2026-06-11 by Malik. Scope (his call): **everything — UI bugs AND agent behavior**. Land on a branch off `main`; **human merges** (per CLAUDE.md). Reuse `pr245`, do not reinvent.

## Discipline (CLAUDE.md / WORKFLOW.md)
- Ticket gate: each bug tracked by a GitHub issue with acceptance criteria (tracking issue created — see PROGRESS).
- TDD: write/extend the failing test first, then fix. Two-layer testing; `make all` (or `go test ./... && golangci-lint run`) green before any PR.
- Atomic commits, format `NNN_short_description`. One logical fix per commit.
- Agents never merge. End state = green PR awaiting Malik.
- Forward-compatible `Opportunity` schema + `Store` interface. Keep code legible.

## Context that explains MOST of the pain
The dashboards Malik tested all run **stub agents** (no `-live-writer`/`-live-review`):
Tomás emits `[Stub draft … KAI-9]`; Vera is deterministic-only. So request-changes shows no
visible change, Vera bounces back to the gate (stub draft fails compliance), and must-haves
never satisfy. The real flow needs **live agents / no-stub failover**, already built on `pr245`
(`1a1afc4 fallback: real-model failover for Writer + Final Review (no stubs)` atop `#243
live pipeline default gemini-3.1-pro`). Bring that onto this branch.

## Bugs + root causes (file references on `main`; re-confirm line numbers per iteration)

### B1 — Workspace sidebar shows 0/0/0 (HIGH, pure UI)
`internal/dashboard/proposals.go` `handleWorkspace`: builds `shellData{PageTitle, ActiveNav}`
and never sets `QueueCount/NeedsCount/ActiveCount` (+`SubmittedCount` once new-app-design merges).
Every other page populates them — copy the pattern from `handleDetail`
(`handler.go`, "Shell counts for the sidebar") / `handleProposals`.
**AC:** on `/workspace/{id}` the sidebar shows the same counts as `/` and `/proposals`. Add a handler test asserting counts > 0.

### B2 — Cross-view state drift / "stuck spinning" (HIGH)
Proposals list derives stage from `DeriveStage(opp)` (via `rowStatus`), the workspace from raw
`opp.ProposalStatus` (`proposalView`). They disagree, so the list can show "ready for review"
while the workspace spins on "Tomás working". Make them derive from one source of truth.
**AC:** list card state == workspace state for the same proposal, across all statuses. Test the two derivations agree.

### B3 — `draft.md` / `document.json` surfaced as artifacts (MEDIUM, UX)
`internal/dashboard/proposals_templates.go` (~"What Tomás produced") hard-renders two
non-interactive `<span>`s with raw internal filenames. Replace with something meaningful
(e.g. a real download/open action for the draft, drop the raw `document.json`), or remove.
**AC:** no raw internal filename shown as a dead label; if an artifact is shown it is actionable.

### B4 — Gate actions give no feedback / "nothing went through" (HIGH)
`handleAction("changes"|"approve")` → `RequestChanges`/`Approve` run, then 303 redirect to the
workspace with no confirmation. With stubs the redraft/review is invisible, reading as a no-op.
Two parts: (a) **feedback** — surface a flash/toast confirming "Sent back to Tomás" / "Vera is
reviewing"; (b) **real progress** — with live agents the redraft/review actually changes the draft.
**AC:** after Request changes, user sees confirmation and the draft/version changes; after Approve,
Vera runs on live content and can reach ready-to-submit (not auto-bounce on stub text).

### B6 — Criteria check false-negative: "must-have missing" when it's in the draft (HIGH)
`internal/dashboard/proposals.go` `deriveCriteria` does
`strings.Contains(strings.ToLower(doc.Markdown()), strings.ToLower(req))` — a verbatim
full-phrase substring match. Any requirement not copied word-for-word (e.g. req
"FedRAMP High authorization" vs draft "FedRAMP High authorized tooling") falsely shows
"Not yet addressed", misleading the human at the go/no-go gate. Reported by Malik 2026-06-11.
**Fix:** replace verbatim phrase match with keyword/token-overlap scoring (e.g. the
requirement's significant terms appearing in the draft), and soften copy so an unconfirmed
item reads "Kaimi couldn't auto-confirm — verify" rather than asserting it's missing. Keep it
honest/derived (no fabrication). **AC:** a draft that addresses a requirement in different
words is NOT flagged missing; table-driven test covers verbatim, paraphrase, and genuinely-absent.

### B5 — Agent behavior: go live / no-stub failover (HIGH, reuse pr245)
Bring `pr245`'s real-model failover for Writer + Final Review onto this branch (cherry-pick
`1a1afc4` + its `#243` base, or rebase). Default the dashboard to live agents with graceful
fallback so the gate flow completes end-to-end. Needs Vertex ADC (`kaimi-seeker`, us-east4).
**AC:** launched without `-live-*` flags the pipeline still uses real models w/ fallback;
a selected opp can reach `final-review:ready_to_submit` with non-stub content.

## Verification (every iteration that touches behavior)
Build + run live, drive with gstack-browse, screenshot before/after:
```
cd C:\Users\Owner\Kaimi-qa-fix
go build -o bin/dash.exe ./cmd/dashboard
copy the real-store:  xcopy /E/I  ..\OneDrive\Documents\Builder\Kaimi\real-store  .\qa-store
.\bin\dash.exe --store=.\qa-store --live-writer --live-review --port=8930   # add creds via gcloud ADC
```
Browse binary: `~/.claude/skills/gstack/browse/dist/browse` (screenshots must land under Pulse or %TEMP%).
Flow to verify: `/` → open opp → Select → `/workspace/{id}`: sidebar counts, draft fills, gate
appears, Request changes (feedback + redraft), Approve (Vera → ready), Submit. Check `/proposals`
card state matches the workspace at each step. Re-check `:8907`/`:8913` parity (web == desktop).

## PROGRESS (loop updates this every iteration)
- [x] Tracking GitHub issue created with AC — Mawar2/Kaimi#246
- [x] B1 workspace sidebar counts — fixed + test + verified (commit 7107c71)
- [x] B2 cross-view state drift — fixed + test + verified (commit 64a278a)
- [x] B3 draft.md/document.json artifacts — fixed + verified (commit 293921f)
- [~] B4 gate-action feedback — DONE + verified (commit 580ed57); "real redraft/review changes the draft" half rides on B5 (live agents)
- [x] B6 criteria false-negative (keyword match + honest copy) — fixed + test + verified (commit 96c2250)
- [ ] B5 live/no-stub failover (pr245 reuse) — landed + verified end-to-end
- [ ] `make all` green (build + test + lint)
- [ ] PR opened for Malik to merge (references issues)

### Iteration log
(append: date — what changed — commit — verify result)
- 2026-06-11 — B1: extracted `fillShellCounts`, wired handleWorkspace + handleDetail. TDD red→green (TestWorkspaceSidebarShowsCounts). Full dashboard pkg green, gofmt/vet clean. Live verify on :8930: workspace sidebar now Opportunities 5 / Proposals 1, matching `/` and `/proposals` (was 0/0). commit 7107c71. Screenshot B1-workspace-sidebar-fixed.png.
- 2026-06-11 — B2: added `ProposalStatus` to OpportunityRow, list now uses `proposalView(row.ProposalStatus)` (same as workspace), removed lossy `rowStatus`. TDD red→green (TestListAndWorkspaceAgreeOnState). Full pkg green, gofmt/vet clean. Live verify on :8930: an outline:failed proposal shows "Needs attention · Outline hit a problem" in BOTH list and workspace; contradiction strings ("Agents working"/"Tomás drafting now") absent. commit 64a278a. Screenshot B2-proposals-failed-consistent.png.
- 2026-06-11 — B4 (feedback half): gate actions redirect with ?flash= marker; workspace renders a one-shot confirmation banner via gateFlashMessage + .ws-flash. TDD red→green (TestGateActionFeedback: redirect Location + banner). Full pkg green, gofmt/vet clean. Live verify on :8930: ?flash=changes shows "Sent back to Tomás…" banner. commit 580ed57. Screenshot B4-gate-feedback-banner.png. Real-redraft-changes-draft half tracked under B5.
- 2026-06-11 — B3: added GET /workspace/{id}/draft.md (text/markdown attachment via handleDraftDownload), template now renders draft.md as a real download link and drops the document.json dead label. TDD red→green (TestDraftDownloadArtifact). Full pkg green, gofmt/vet clean. Live verify on :8930: endpoint returns 200 text/markdown, Content-Disposition attachment "90MC26R0004-draft.md", real draft body. commit 293921f.
- 2026-06-11 — B6: replaced verbatim `strings.Contains` criteria check with significant-term overlap + light stemming (`requirementAddressed`/`stemTerm`), honest "could not auto-confirm" copy. TDD red→green: criteria_test.go (6-case table) + TestGateCriteriaMatchesParaphrase (renders gate: paraphrase=met, absent=honest copy). Full pkg green, gofmt/vet clean. Live browser verify deferred to B5 (reaching the gate needs live agents/creds); proven at unit+integration layer. commit 96c2250.
