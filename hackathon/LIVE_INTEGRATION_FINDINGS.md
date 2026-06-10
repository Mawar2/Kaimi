# Kaimi — Live Integration Findings

**Owner of this file:** live-integration agent
**Last updated:** 2026-06-10 (session 2)
**Working dir:** `C:\Users\Owner\Kaimi-live` (isolated clone, per the integration brief — NOT the build-loop checkout)
**Verified against:** project `kaimi-seeker`, region `us-east4`, account `malik@bluemetatech.com`

This file is the one-way channel from the live-integration agent to the build-loop.
Each finding is tagged **OWNER: build-loop** (agent-logic fix — the build-loop fixes it on a
clean net-new branch and merges) or **OWNER: live-agent** (config/data/infra — mine to fix).
Live agent does **not** edit `internal/{manager,outline,writer,finalreview,ingest,opportunity,scorer,store,agent}`.

---

## ⭐ Session-2 summary (2026-06-10) — the FULL product was driven live on a REAL solicitation

Without waiting for the SAM quota, we sourced a real opportunity manually and ran the entire product
end-to-end on live agents. **Every stage now proven on real federal data:**

1. **Real opportunity sourced + seeded** (`cmd/demo-seed`, PR #193): a research sub-agent found a
   verified, eligible SAM.gov notice — **Selective Service System Website Modernization** (`90MC26R0004`,
   notice `e89891bf…`, NAICS 541519, Total Small Business Set-Aside, due 2026-06-30). Ported into the
   store through the **real DeterministicScorer** (same path as `cmd/pipeline`) → **score 0.78, BID**.
   This replaces F6 — the store now holds a genuinely real, eligible opportunity (no SAM API call).
2. **Full live Zone-2 chain** ran on it (Outline → Writer → gate → approve → Final Review), with the
   Writer grounding the draft in the real notice (cited Amendments 0001/0002, FFP award, FAR 52.212-1).
3. **Document ingestion proven live** (`-live-ingest`): the 4 real solicitation PDFs/DOCX were fetched
   → stored to `gs://kaimi-seeker-solicitations/{noticeID}/raw/` → **OCR'd by a real Document AI
   processor** → text threaded into the Writer + the live Final Review compliance pass. (See INGEST below.)
4. **Human-in-the-loop gate verified end-to-end → `submitted`**: acting as the human capture lead, drove
   Request-Changes → direct section edit → Approve → Submit. Human-only Submit confirmed. (See HUMAN GATE.)
5. **Writer grounded on BlueMeta's real profile** (`config/bluemeta_scorer_profile.json`, PR #196) — no
   more fabricated agencies.

**New issues filed:** #192 (F2 — thinking tokens, all 3 LLM agents) · #194 (Document AI large-PDF +
DOCX routing) · #195 (Request-Changes note not passed to Writer).
**New PRs (await Malik's merge):** #193 (demo-seed) · #196 (BlueMeta profile). Plus session-1 #185, #186.

**The single remaining blocker for a flawless all-live demo is #192 (F2).**

**Infra provisioned this session (live-agent):** Document AI OCR processor `93d4245d53e97106` (loc `us`);
granted the ADC identity bucket-write + Document AI use. ⚠️ **Two-identity gotcha:** gcloud CLI = `malik@bluemetatech.com`
(owner) but **ADC = `malikpwarren@gmail.com`** (only `aiplatform.admin` + `viewer`) — REST/Go-SDK calls
use ADC, so storage/Document AI writes 403 until that identity is granted the role (via the owner CLI).

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

**Filed as issue #192.** **Confirmed to break ALL THREE LLM agents** (live, 2026-06-10):
- **Scorer (`scorer.go:143`, 1024, JSON schema): FAILS** — complex prompt + thinking → empty/truncated
  JSON. *Not in the judge path* if opportunities are pre-scored (they are), but blocks Zone-1 live seeding.
- **Writer (`writer/gemini.go:50`, 2048, per-section): MOSTLY WORKS** — ~5 of 7 sections draft fully; the
  two longest **truncate mid-sentence** (MAX_TOKENS). Raising the cap / capping thinking fixes it.
- **Final Review (`finalreview/gemini.go:45`, 4096, JSON): FAILS with documents** — once the live
  compliance pass runs against real ingested solicitation text, the response truncates and the agent
  flags `[compliance_error] compliance response could not be parsed (verify manually)`. It degrades
  gracefully (→ `needs_human`) but the live verdict is blocked.
- Severity for an all-live demo: **this is THE blocker.** The Outline+Writer judge path is demo-usable,
  but a clean Scorer score, untruncated Writer sections, and a real compliance verdict all need this fix.

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

### F5 — `GCS_SOLICITATIONS_BUCKET` wiring — ✅ RESOLVED (#172/#174 merged + `-live-ingest`)
**OWNER: build-loop — DONE** (was: bucket referenced in a comment, never read)
The build-loop merged #172/#174, and `cmd/dashboard` now has a **`-live-ingest`** flag that constructs
the ingestor from env (`GCS_SOLICITATIONS_BUCKET`, `DOCUMENTAI_PROCESSOR_ID`, `DOCUMENTAI_LOCATION`) and
passes it to `proposal.Service.Ingest`. Verified live (see INGEST below). No further action.

### F6 — Demo dataset — ✅ RESOLVED (real opportunity seeded via `cmd/demo-seed`, PR #193)
**OWNER: live-agent — DONE** (was: store held only synthetic fixtures)
The store now holds a **real, eligible** opportunity (Selective Service `90MC26R0004`, notice
`e89891bf…`) sourced from SAM.gov and scored by the real DeterministicScorer (0.78, BID) — see the
session-2 summary. The synthetic `a1b2c3d4e5f6`/`9z8y7x6w5v4u` fixtures are no longer the demo data.
(Fresh *Gemini-scored* data still needs one live SAM pull after F1 clears + F2 fixed.)

### F7 — Ingest: large PDFs fail Document AI sync OCR; DOCX mis-routed when served as octet-stream
**OWNER: build-loop** — filed as **issue #194**
Live `-live-ingest` run on the real Selective Service docs: all 4 fetched + stored to GCS, but **2 of 4
produced no text**. (1) The **2.7 MB package PDF** (the one with Section L/M) exceeded Document AI's
**synchronous** `ProcessDocument` page limit → no text; `internal/ingest/documentai.go` uses the sync
path, so large solicitations silently yield nothing — needs **batch** processing. (2) The `.docx` was
served as `application/octet-stream`, so `internal/ingest/extractor.go`'s content-type-only routing sent
it to OCR instead of the in-Go DOCX extractor — needs an **extension fallback**. The two Amendment PDFs
OCR'd correctly, proving the fetch→store→extract→thread pipeline works.

### F8 — Human-gate "Request Changes" note is never passed to the Writer
**OWNER: build-loop** — filed as **issue #195**
The gate's Request-Changes action records the note and re-runs the Writer, but `proposal.Service.runRevision`
rebuilds `writer.Input{Opportunity, Outline, Profile}` with **no feedback field** — so the revision just
re-rolls the same prompt. Verified live: a capture-lead note to fix fabricated past performance was ignored
by the revision (had to fix via direct section edit). The rest of the human gate works (see HUMAN GATE).

### F9 — Writer grounded on the generic fixture profile, not BlueMeta — ✅ FIXED by live-agent (PR #196)
**OWNER: live-agent — DONE**
The dashboard defaulted to `test/fixtures/capability_profile.json` (a generic federal-IT profile listing
DoD/DHS/VA/GSA/HHS past performance), so the Writer **asserted past performance for agencies BlueMeta never
served**. Fixed by adding `config/bluemeta_scorer_profile.json` (BlueMeta's real profile) and wiring it as
`cmd/dashboard`'s default `--profile` (PR #196). After the fix the fabricated agencies are gone; the Writer
honestly flags `[GAP]` for formal details it lacks. **Related build-loop nuance:** in the Past Performance
section the Writer templates a formal "Project Reference" block and gaps the *whole* block (incl. client
name) when contract numbers/values aren't in the profile, even though real client names are present — worth
a Writer-prompt revisit.

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

## ✅ INGEST + Document AI VERIFIED on the real solicitation (2026-06-10)

Ran `cmd/dashboard --live-writer --live-review --live-ingest` (env: `GCS_SOLICITATIONS_BUCKET=kaimi-seeker-solicitations`,
`DOCUMENTAI_PROCESSOR_ID=93d4245d53e97106`, `DOCUMENTAI_LOCATION=us`) on the seeded Selective Service
opportunity, with the 4 real downloaded docs served locally (SAM attachments need auth, so the HTTP
fetcher pulled from a localhost file server — a stand-in for the real SAM URLs).

- ✅ All 4 attachments fetched → `gs://kaimi-seeker-solicitations/{noticeID}/raw/` → attached as
  `SolicitationDoc`s on the opportunity.
- ✅ **Document AI OCR produced real, accurate text** for the two Amendment PDFs (verified: correctly read
  the SF30 form — "AMENDMENT OF SOLICITATION… 90MC0026R00040002… Arlington VA"). Text written to `…/text/`.
- ✅ Text threaded into the Writer and the **live Gemini Final Review compliance pass** (which then ran
  ~35s analyzing real document text, vs ~5s deterministic without docs).
- ❌ 2 of 4 docs produced no text — see **F7/#194** (large package PDF over sync OCR limit; DOCX mis-routed).
- ❌ The compliance verdict itself truncated — see **F2/#192** (3rd affected agent).

## ✅ HUMAN-IN-THE-LOOP GATE VERIFIED end-to-end → `submitted` (2026-06-10)

The human gate is a **product feature**, not a stopping point. Acting as the human capture lead, drove
every gate action over HTTP and reached the terminal `submitted` state:

| Human action | Endpoint | Result |
| --- | --- | --- |
| Review draft | `GET /workspace/{id}` | caught fabricated past performance (F9) |
| **Request Changes** | `POST /workspace/{id}/changes` (`note`) | Writer re-ran → back to gate (note ignored — **F8/#195**) |
| **Edit section** | `POST /workspace/{id}/section/{sid}` (`body`) | edit persisted ✓ |
| **Approve** | `POST /workspace/{id}/approve` | → Final Review → `ready_to_submit` ✓ |
| **Submit** | `POST /workspace/{id}/submit` | **`submitted`** ✓ (human-only; Kaimi never auto-submits) |

Every status transition persisted; Submit is gated on `ready_to_submit` and is a human-only terminal act.
The gate works as designed — the one gap is F8 (the change note doesn't reach the Writer).

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

- [x] GCP runtime verified: ADC auth, SAM key (Secret Manager), Vertex/Gemini reachable, Document AI processor.
- [x] GCS bucket + IAM for solicitation documents (#162) — provisioned, verified, PR #185.
- [ ] Zone-1 live against real SAM.gov — **blocked by F1** (quota; retry after 2026-06-11 00:00 UTC).
- [x] A REAL opportunity seeded (Selective Service `90MC26R0004`) via the real scorer — `cmd/demo-seed`, PR #193 (replaces F6).
- [x] Full Zone-2 chain runs **live end-to-end** on the real opportunity (Outline+Writer+Final Review).
- [x] **Document ingestion + Document AI** verified live (real docs OCR'd + threaded) — partial extraction, F7/#194.
- [x] **Human-in-the-loop gate** verified end-to-end → `submitted` (review/request-changes/edit/approve/submit).
- [x] Writer grounded on **BlueMeta's real profile** by default — PR #196 (fixes F9).
- [x] Dashboard renders the live store (HTTP 200).
- [x] This findings file exists and is current.
- [x] Infra/config changes committed on `integration/*` with PRs opened.

## Open PRs (await Malik's merge — live-agent merged nothing)
| PR | What | Branch |
| --- | --- | --- |
| #185 | GCS solicitations bucket + IAM (#162) | `integration/gcs-setup` |
| #186 | This findings doc | `integration/live-findings` |
| #193 | `cmd/demo-seed` — real opportunity via real scorer | `integration/demo-seed` |
| #196 | BlueMeta scorer profile + dashboard default (F9) | `integration/bluemeta-profile` |

## Open issues for the build-loop
| Issue | What | Severity |
| --- | --- | --- |
| **#192** | **F2 — gemini-2.5-pro thinking tokens truncate Scorer/Writer/Final-Review output** | **demo blocker** |
| #194 | F7 — large PDFs fail sync Document AI OCR; DOCX mis-routed | medium |
| #195 | F8 — Request-Changes note never reaches the Writer | medium |

## Recommended sequence to unblock the win (for Malik / build-loop)
1. **build-loop:** fix **#192 (F2)** — the one demo blocker. Without it, the live Scorer/Writer/Compliance
   all truncate. Everything else is already proven live.
2. **build-loop (polish):** #194 (batch OCR for the big package PDF so compliance sees Section L/M) and
   #195 (thread the human change note into the Writer).
3. **Malik:** merge the 4 integration PRs above.
4. **live-agent:** at/after 2026-06-11 00:00 UTC, run `cmd/pipeline --mode=live` for fresh Gemini-scored
   opportunities (F1); for the demo, the seeded Selective Service opportunity already works end-to-end.
