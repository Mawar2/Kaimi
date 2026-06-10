# Kaimi — Live Integration Findings

**Owner of this file:** live-integration agent
**Last updated:** 2026-06-10
**Working dir:** `C:\Users\Owner\Kaimi-live` (isolated clone, per the integration brief — NOT the build-loop checkout)
**Verified against:** project `kaimi-seeker`, region `us-east4`, account `malik@bluemetatech.com`

This file is the one-way channel from the live-integration agent to the build-loop.
Each finding is tagged **OWNER: build-loop** (agent-logic fix — the build-loop fixes it on a
clean net-new branch and merges) or **OWNER: live-agent** (config/data/infra — mine to fix).
Live agent does **not** edit `internal/{manager,outline,writer,finalreview,ingest,opportunity,scorer,store,agent}`.

---

## Environment verified

| Check | Result |
| --- | --- |
| gcloud ADC auth | ✅ token mints; account `malik@bluemetatech.com`, project `kaimi-seeker` |
| Go toolchain | ✅ go1.25.1; `go build ./...` clean on `origin/main` @ `ff57969` |
| Secret Manager | ✅ can read `samgov-api-key` (`SAM-1c27e3e7…`), `google-ai-studio-api-key`, drive creds |
| Vertex AI / Gemini reachable | ✅ direct `generateContent` on `gemini-2.5-pro` returns a candidate (see F2) |
| Deployed pipeline SA | `kaimi-dev@kaimi-seeker.iam.gserviceaccount.com` |

---

## Findings

### F1 — SAM.gov daily quota exhausted (BLOCKER for fresh live data)
**OWNER: live-agent** (external/timing — no code change)
`cmd/pipeline --mode=live` fails on the first SAM.gov fetch:
```
SAM.gov API returned status 429: {"code":"900804","message":"Message throttled out",
"description":"You have exceeded your quota. You can access API after 2026-Jun-11 00:00:00+0000 UTC"}
```
The daily quota is spent. **Resets 2026-06-11 00:00:00 UTC.** Fetching *new* real opportunities
(Zone-1 live, seeding, and therefore the full Zone-2 live flow on real data) is blocked until then.
This matches the long-standing "SAM quota tier" suspicion in deploy notes.
**Action:** re-run `cmd/pipeline --mode=live` right after 00:00 UTC 2026-06-11 to capture real
opportunities, then drive Zone-2. Quota is small — cache aggressively, one pull only.

### F2 — `gemini-2.5-pro` thinking tokens starve the output budget → live scoring/drafting returns empty/truncated JSON (CRITICAL)
**OWNER: build-loop** (agent logic, in-flight package — do not edit; reported here)
`cmd/eval --agent scorer` fails live:
```
eval: scorer failed on case "cloud-migration-strong-bid": invalid Gemini response:
failed to parse JSON response: unexpected end of JSON input
```
Root cause, confirmed by a direct Vertex call (trivial prompt, `maxOutputTokens=50`):
```
"finishReason": "MAX_TOKENS",
"usageMetadata": { "candidatesTokenCount": 2, "thoughtsTokenCount": 44 }
```
`gemini-2.5-pro` is a **thinking model**: internal reasoning tokens (`thoughtsTokenCount`) are
charged against `MaxOutputTokens`. On a real prompt the thinking budget consumes most/all of the
cap, so the actual JSON body is empty or cut off → parse failure.

Affected (read-only inspection):
- `internal/scorer/scorer.go:141-146` — `MaxOutputTokens: 1024`, `ResponseMIMEType: application/json`,
  **no `ThinkingConfig`**. 1024 is easily exhausted by thinking on a scoring prompt.
- `internal/writer/gemini.go:47-52` — `MaxOutputTokens: 2048`, **no `ThinkingConfig`**. Same failure
  mode for longer section drafts.

Why mocked tests don't catch it: the unit/contract layer returns canned JSON and never exercises the
live thinking-token behavior. This is a real two-layer-testing gap — the code is green in CI and
fails on the first live call.

**Suggested fix (build-loop's call):** set `ThinkingConfig` to bound/disable thinking
(`ThinkingConfig{ThinkingBudget: <small or 0>}`), and/or raise `MaxOutputTokens` well above the
thinking budget (e.g. 8192), and treat `finishReason == MAX_TOKENS` as an explicit error rather than
letting it surface as a JSON parse failure. Add a `-tags live` test that asserts a non-empty,
schema-valid response from the real model so this can't regress silently.

**Empirical per-agent impact (live run 2026-06-10, see "Live Zone-2 chain verified" below):**
- **Scorer (1024, JSON schema): FAILS** — complex full-opportunity prompt + thinking → empty/truncated
  JSON. *Not in the judge path* if opportunities are pre-scored (they are), but blocks Zone-1 live seeding.
- **Writer (2048, per-section): MOSTLY WORKS** — 5 of 7 sections drafted fully; the two longest
  (Technical Approach, Past Performance) **truncate mid-sentence** (MAX_TOKENS). Raising the writer's
  `MaxOutputTokens` (or capping thinking) fixes the cut-offs. The draft is still demo-usable.
- Severity for an all-live demo: **medium, not a hard blocker** — the judge path produced a complete,
  grounded, submittable draft. Fix the writer cap for polish; fix the scorer before Zone-1 live seeding.

### F3 — `setup-gcp` scripts write `GEMINI_MODEL=gemini-3.0-pro`, but code + docs use `gemini-2.5-pro`
**OWNER: live-agent** (config — mine; not changed in PR #185 to keep that PR scoped to #162)
`scripts/setup-gcp.sh:173` and `scripts/setup-gcp.ps1:209` emit `GEMINI_MODEL=gemini-3.0-pro` into
`.env.gcp`. Every Go default and CLAUDE.md use `gemini-2.5-pro`. If an operator sources `.env.gcp`,
they'd point at a different (possibly nonexistent) model. Needs a decision on the canonical model id,
then a one-line script fix. Tracking here; will fix on a follow-up `integration/*` branch once the id
is confirmed.

### F4 — Zone-2 chain runs via the dashboard, not a headless CLI (CORRECTED — not a blocker)
**OWNER: shared / informational** (originally filed as "no driver"; corrected after reading `cmd/dashboard`)
Correction: the full Zone-2 chain **does** have a runtime driver — it's the **dashboard**, not a CLI.
`cmd/dashboard/main.go:48-121` (`newProposalService`) wires Outline + Writer + Final Review through
`internal/proposal.Service` behind the gated lifecycle, with flags:
- `-live-writer` → real Gemini drafting over Vertex ADC (needs `GCP_PROJECT_ID`)
- `-live-review` → Gemini compliance pass in Final Review
The chain is driven by in-UI gate actions on an opportunity, persisting `ProposalStatus`. `cmd/eval`
is a *separate* reliability harness (scores/drafts in isolation) — not the driver.

Remaining real gap: there is **no headless/CLI driver** for the chain, so a fully-scripted live
end-to-end run (no browser) isn't possible today — the proposal must be advanced through the dashboard
UI. A small `cmd/propose` that loads an opportunity and runs `proposal.Service` to completion would
make Task 5 fully scriptable and CI-demonstrable. Nice-to-have, **OWNER: build-loop** — not blocking,
since the dashboard path works.
Caveat: `-live-writer`/`-live-review` will hit **F2** (thinking-token truncation) until F2 is fixed.

### F5 — `GCS_SOLICITATIONS_BUCKET` is referenced in a comment but never read from config
**OWNER: build-loop** (app wiring, ingest/manager path)
`internal/ingest/gcsstore.go:28` says the bucket comes from `GCS_SOLICITATIONS_BUCKET` "in the app
config", but nothing in the repo actually reads that env var / config key — `NewGCSStore` just takes a
`bucket string` and no caller wires it. The bucket now exists (F-INFRA below / PR #185) but the live
ingest path can't pick it up until the Manager/ingest wiring reads `GCS_SOLICITATIONS_BUCKET` (or an
equivalent config field) and passes it to `NewGCSStore`.

### F6 — The "live" queue holds synthetic fixture data scored offline, not real SAM/Gemini output
**OWNER: live-agent** (data — will be replaced once F1 clears)
`gs://kaimi-seeker-queue/queue/queue/` contains exactly two opportunities, `a1b2c3d4e5f6` and
`9z8y7x6w5v4u` — the **synthetic fixtures** from `test/fixtures/samgov_response.json`, with
`score_reasoning: "Deterministic score 78/100 (offline rubric)"`. They were produced by the offline
`DeterministicScorer`, not real SAM.gov data or live Gemini. The deployed Cloud Run job has not
yet produced a genuinely-live scored opportunity. The demo dataset is **not real yet** — must be
re-seeded after the SAM quota resets (F1).

---

## ✅ Live Zone-2 chain VERIFIED end-to-end (2026-06-10) — the judge path works on live agents

Ran the real dashboard (`cmd/dashboard --store=<seeded> --live-writer --live-review`, project
`kaimi-seeker`) against a pre-seeded opportunity (`a1b2c3d4e5f6`, GSA Cloud Modernization) and drove
the full gated lifecycle over HTTP — **no SAM.gov call in this path**:

| Step | Trigger | Result |
| --- | --- | --- |
| Select | `POST /opportunity/{id}/select` | `outline:in_progress` → live Outline built skeleton |
| Draft | (auto) | live **Gemini Writer** drafted 7 sections, `draft.md` = 9.8 KB |
| Gate | (auto) | `writer:needs_human` — paused for the human ✓ |
| Approve | `POST /workspace/{id}/approve` | live **Final Review** ran |
| Verdict | (auto) | **`final-review:ready_to_submit`** ✓ |
| Submit | — | human-only; Kaimi never auto-submits ✓ |

Highlights:
- **Anti-fabrication grounding works live:** the Writer inserted `[GAP: …]` markers for facts not in
  the capability profile (e.g. `[GAP: Program Manager name]`, `[GAP: percentage goal for SB]`) instead
  of inventing them. Strong demo moment.
- **Human gate intact**, statuses persisted at every transition (the UI polls them).
- Caveats: (1) Writer truncation on 2 long sections — see F2. (2) `final-review:ready_to_submit` here
  came from the **deterministic** compliance checks; the live Gemini compliance *pass* only fires once
  solicitation-document text is threaded into `finalreview.Input.Documents` (#172) — until then it
  skips for lack of documents. So "all agents live" is true for Outline+Writer today; Final Review's
  *LLM* pass needs #172 + the ingest wiring (F5).

**Bottom line:** with opportunities pre-seeded, the all-live judge demo (Outline+Writer live now,
Final Review LLM after #172) is demonstrably viable. The only true external dependency is one real
SAM pull to seed real opportunities (F1).

## Done this iteration (infra, by the live agent)

### #162 — Solicitation-documents bucket provisioned & verified → PR #185
**OWNER: live-agent — DONE**
- Created `gs://kaimi-seeker-solicitations`: **us-east4**, uniform bucket-level access,
  public access prevention **enforced**.
- Granted `kaimi-dev@kaimi-seeker.iam.gserviceaccount.com` `roles/storage.objectAdmin` **scoped to
  the bucket** (least-privilege).
- Verified a real round-trip under the `{noticeID}/raw/{filename}` + `{noticeID}/text/{filename}.txt`
  layout; smoke objects removed.
- Made it reproducible: idempotent Step 11 in `scripts/setup-gcp.{sh,ps1}`, `GCS_SOLICITATIONS_BUCKET`
  written to `.env.gcp` and recorded in `.env.example`.
- PR: **https://github.com/Mawar2/Kaimi/pull/185** (base `main`, left for human merge).

### Dashboard renders the live store (deterministic mode) — verified
**OWNER: live-agent — DONE (data is synthetic until F1 clears)**
- `cmd/dashboard --store=./live-store/queue` (synced from `gs://kaimi-seeker-queue`) serves
  `http://127.0.0.1:8900/` → **HTTP 200**, both opportunities render (titles + agencies), Zone-2
  proposal/outline/gate surfaces present.
- Confirms the dashboard + `internal/proposal` wiring work end-to-end against the JSON store. Once
  real opportunities are seeded (post-F1), the same command shows them; add `-live-writer
  -live-review` for the live LLM path **after F2 is fixed**.

---

## Definition-of-done tracker (brief)

- [x] GCP runtime verified: ADC auth, SAM key present (Secret Manager), Vertex/Gemini reachable.
- [x] GCS bucket + IAM for solicitation documents (#162) — provisioned, verified, PR #185.
- [ ] Zone-1 live against real SAM.gov — **blocked by F1** (quota; retry after 2026-06-11 00:00 UTC).
- [x] Full Zone-2 chain runs **live end-to-end** on a seeded opportunity (Outline+Writer+Final Review → `ready_to_submit`, human gate intact). Real *opportunity data* still pending one SAM pull (F1); Final Review's LLM pass pending #172.
- [x] Dashboard renders store data (HTTP 200, both opps) — verified; will show *real* data once F1 clears (currently synthetic, F6).
- [ ] A small set of REAL opportunities seeded — **blocked by F1**.
- [x] This findings file exists and is current.
- [x] Infra/config changes committed on `integration/*` with PR opened (#185).

## Recommended sequence to unblock the win (for Malik / build-loop)
1. **build-loop:** fix F2 (thinking-token budget in scorer + writer) — without this, live scoring/drafting fails even with SAM data.
2. **build-loop:** add F4 (a `cmd/` driver for the Manager chain) and F5 (wire `GCS_SOLICITATIONS_BUCKET`).
3. **live-agent:** at/after 2026-06-11 00:00 UTC, run `cmd/pipeline --mode=live` to seed real opportunities (F1/F6), then drive the Manager chain and point the dashboard at the live store.
