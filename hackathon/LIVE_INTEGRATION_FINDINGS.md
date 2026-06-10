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

### F3 — `setup-gcp` scripts write `GEMINI_MODEL=gemini-3.0-pro`, but code + docs use `gemini-2.5-pro`
**OWNER: live-agent** (config — mine; not changed in PR #185 to keep that PR scoped to #162)
`scripts/setup-gcp.sh:173` and `scripts/setup-gcp.ps1:209` emit `GEMINI_MODEL=gemini-3.0-pro` into
`.env.gcp`. Every Go default and CLAUDE.md use `gemini-2.5-pro`. If an operator sources `.env.gcp`,
they'd point at a different (possibly nonexistent) model. Needs a decision on the canonical model id,
then a one-line script fix. Tracking here; will fix on a follow-up `integration/*` branch once the id
is confirmed.

### F4 — No runtime entrypoint drives the full Zone-2 chain (Manager → Outline → Writer → Final Review)
**OWNER: build-loop** (needs a `cmd/` driver or documented invocation)
`cmd/` has: dashboard, eval, hunter, outline-probe, pipeline, scorer, spike. None of them run the
`internal/manager` conductor over a selected opportunity. `cmd/eval` only scores/drafts in isolation
(reliability harness), it does not thread an opportunity through the Manager. There is therefore no
way to execute "human selects → Manager → Outline → Writer → Final Review" live from the CLI.
**Blocks** the brief's Task 5 (prove the full proposal flow end to end) independent of the SAM quota.
**Suggested:** add a small `cmd/manager` (or `cmd/propose`) that loads an opportunity from the Store
and runs the Manager chain, persisting `ProposalStatus`. (Calling the in-flight packages is fine; the
driver itself is runtime wiring.)

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

---

## Definition-of-done tracker (brief)

- [x] GCP runtime verified: ADC auth, SAM key present (Secret Manager), Vertex/Gemini reachable.
- [x] GCS bucket + IAM for solicitation documents (#162) — provisioned, verified, PR #185.
- [ ] Zone-1 live against real SAM.gov — **blocked by F1** (quota; retry after 2026-06-11 00:00 UTC).
- [ ] Real opportunity through the full Zone-2 chain — **blocked by F1 + F4** (no Manager driver) **+ F2** (live LLM truncation).
- [ ] Dashboard renders live data — pending (queue currently holds synthetic data, F6).
- [ ] A small set of REAL opportunities seeded — **blocked by F1**.
- [x] This findings file exists and is current.
- [x] Infra/config changes committed on `integration/*` with PR opened (#185).

## Recommended sequence to unblock the win (for Malik / build-loop)
1. **build-loop:** fix F2 (thinking-token budget in scorer + writer) — without this, live scoring/drafting fails even with SAM data.
2. **build-loop:** add F4 (a `cmd/` driver for the Manager chain) and F5 (wire `GCS_SOLICITATIONS_BUCKET`).
3. **live-agent:** at/after 2026-06-11 00:00 UTC, run `cmd/pipeline --mode=live` to seed real opportunities (F1/F6), then drive the Manager chain and point the dashboard at the live store.
