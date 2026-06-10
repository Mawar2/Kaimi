# Kaimi — End-to-End Backlog (Missing Tickets)

**Last updated:** 2026-06-09
**Author:** Malik (capture lead) — drafted for team review
**Status:** ⚠️ **SUPERSEDED / LARGELY STALE (2026-06-09).** This was an early-morning
snapshot. Since it was written, the gap it describes has been **closed**: the full
chain (Hunter → Scorer → Queue, and the Zone-2 Manager → Outline → Writer → Final
Review packages) is **built**, and the Zone-1 pipeline is **deployed** (Cloud Run Job
`kaimi-pipeline` on Cloud Scheduler, JSON store in `gs://kaimi-seeker-queue`). Do
**not** trust the "nothing runs end to end" framing below or the per-ticket "OPEN /
blocked" statuses — verify against GitHub Issues and `internal/` before acting. Kept
only for historical traceability of how the work was scoped. The current focus is
finishing the web + desktop dashboards and submission polish for June 11.

---

## Purpose

A GitHub sweep on 2026-06-09 found that Kaimi's individual agents largely exist
(Hunter, Scorer, Outline, Final Review skeleton, Capability Profile, Opportunity
schema, Store), **but nothing yet runs an opportunity end to end** — from SAM.gov
through scoring, drafting, and review, back to a human. This document captures the
tickets needed to close that gap, in dependency order.

These are **proposals**, not approved work. Per the Hard Gate in CLAUDE.md, no code
is written until each ticket below is filed as a GitHub Issue with **human-approved
acceptance criteria**. When approved, file each in `Mawar2/Kaimi` and copy the queue
entries into `docs/tickets/malik-tickets.md` / `docs/tickets/timm-tickets.md`.

The intended pipeline (ARCHITECTURE.md):

```
Zone 1 (scheduled):           Zone 2 (orchestrated, per-proposal):
Hunter → Scorer ───────────►  Manager → Outline → Writer → Final Review → human approves
```

---

## Definition of Done (applies to EVERY ticket below)

Do not restate per ticket. Every ticket inherits the Universal DoD from CLAUDE.md /
WORKFLOW.md, the important parts being:

- All acceptance criteria met with evidence (`file:line` or test name)
- **TDD**: test written first, watched fail, then code to pass
- Both test layers pass: unit/contract (mocked, fast, every commit) **and** relevant
  E2E (live APIs, run separately)
- `make lint` and `gofmt` clean; `make all` green
- No new deps / files / patterns without justification on the ticket (and
  CONVENTIONS.md updated if a new pattern is introduced)
- Branch `feature/KAI-XXX-short-summary`; commits `XXX_feature_description`
- No secrets committed
- CI passes (tests + lint + AI review completes); **human approves and merges** — no
  agent merges code

---

## Gate 0 — Clear these existing blockers FIRST (not new tickets)

The new work below depends on these. They already have issues; they are stuck.

| Item | Issue / PRs | Why it blocks end-to-end |
|---|---|---|
| **AgentResult contract** | #8 (OPEN; PRs #16/#21/#22/#42/#43 all closed unmerged) | The return type every agent conforms to. The Manager (KAI-M5) and Writer (KAI-8/9) cannot be wired safely until this lands. **Highest priority.** |
| **Final Review actual checks** | #7 (OPEN; ~13 duplicate PRs #60–#85) | The skeleton (#6) is merged; the real must-have / formatting checks are not. Required for the last pipeline stage. |
| **PR hygiene** | ~25 duplicate open PRs across #7, #59, and "file-loadable CapabilityProfile" | The orchestration bot keeps opening fresh PRs for the same three tickets. Pick one PR per ticket, close the rest, before status is legible. |
| **Close merged-but-open** | #11 (Scorer; PR #44 merged but issue still OPEN) | Verify and close, or split out the follow-up. |

---

## New Tickets (dependency-ordered build sequence)

### KAI-8 — Writer agent: skeleton
- **Phase:** 3 · **Zone:** 2 · **Agent:** `agent:writer` (new label) · **Owner:** Timm
- **Depends on:** #8 (AgentResult)
- **Context:** Outline produces the section structure; nothing currently turns it into
  draft prose. Stand up the Writer's shell and prove it fits the interface before
  adding real generation. Mirrors the KAI-2 (Outline skeleton) pattern.

**Done when:**
- Takes an `Opportunity` + its outline (sections + formatting rules) in, returns an `AgentResult`
- Returns `success` on the happy path (stubbed draft output is fine here) and `failed` with an error when it can't
- Runs against the cached test fixture — no live model calls
- New `agent:writer` label created and CONVENTIONS.md noted if a new package `internal/writer` is added

**Not this ticket:** real draft generation (KAI-9), Google Docs, orchestration.

---

### KAI-9 — Writer agent: real drafting
- **Phase:** 3 · **Zone:** 2 · **Agent:** `agent:writer` · **Owner:** Timm
- **Depends on:** KAI-8
- **Context:** The real logic — turn the outline into grounded section drafts using
  Gemini, anchored to the `Opportunity` and the Capability Profile so the draft
  reflects BlueMeta's real facts.

**Done when:**
- Generates draft content for each outline section, grounded in the `Opportunity` and Capability Profile
- **Never fabricates past performance or compliance claims** — only uses facts present in the profile/opportunity; gaps are flagged, not invented
- Respects the formatting rules attached by the Outline agent (KAI-4)
- Returns the draft in the `AgentResult`; `failed` with error if generation can't complete (don't emit a silent empty draft)
- Deterministic tests on structure (sections present, no fabricated fields); E2E against a real model asserts behavior, not exact text

**Not this ticket:** saving to Google Docs (KAI-5), final review checks (#7), submission (always human).

---

### KAI-M5 — Manager agent: Zone 2 per-proposal orchestrator
- **Phase:** 3 · **Zone:** 2 · **Agent:** `agent:manager` (new label) · **Owner:** Malik
- **Depends on:** #8 (AgentResult), Outline (done), KAI-9 (Writer), #7 (Final Review checks)
- **Context:** The missing Zone-2 glue. Given one eligible, scored `Opportunity`, the
  Manager threads it through Outline → Writer → Final Review, persisting each
  `AgentResult` to the Store and halting on failure or human-needed states. This is
  what makes a single proposal run end to end.

**Done when:**
- Accepts one `Opportunity` and runs the Zone-2 chain in order: Outline → Writer → Final Review
- Persists each stage's `AgentResult` to the Store (forward-compatible with the existing schema)
- Halts and surfaces clearly on any stage returning `failed` or `needs_human` — does not silently continue
- **Never auto-submits** — terminal success state is `ready_to_submit` awaiting a human
- Tests cover: full happy path (all stages success), a mid-chain `failed`, and a `needs_human` halt — all against fixtures
- New `agent:manager` label; CONVENTIONS.md updated if a new orchestration pattern is introduced

**Not this ticket:** the Zone-1 scheduler (KAI-M6), Google Docs, the agents' internal logic.

---

### KAI-M6 — Zone 1 scheduled pipeline runner + entrypoint
- **Phase:** 1→3 bridge · **Zone:** 1 · **Owner:** Malik
- **Depends on:** Hunter (done), Scorer (done)
- **Context:** Hunter and Scorer exist as components but nothing runs them as a
  pipeline. Add a `cmd/` entrypoint (e.g. `cmd/kaimi` or `cmd/pipeline`) that runs
  Hunter → Scorer, persisting scored opportunities to the Store. This is the Zone-1
  glue and the first thing an operator actually runs.

**Done when:**
- A single command runs Hunter → Scorer and writes scored `Opportunity` records to the Store
- Supports a single-shot run now; structured so a scheduler can call it later (don't build the scheduler infra yet — `// TODO(phase-N):`)
- Respects SAM.gov rate limits and uses the existing cache aggressively (no unnecessary live calls)
- Cached-mode run works with no API key (fixtures); live mode behind an explicit flag
- Tests cover: cached full Zone-1 run produces scored opportunities in the Store

**Not this ticket:** Agent Engine / cron / cloud scheduling infra (provision lazily, later phase), Zone-2 chain (KAI-M5).

---

### KAI-10 — End-to-end integration test of the Kaimi pipeline
- **Phase:** 3 · **Zone:** 1+2 · **Owner:** Timm + Malik
- **Depends on:** KAI-M5 (Manager), KAI-M6 (Zone-1 runner), KAI-9 (Writer), #7 (Final Review checks)
- **Context:** Proves the whole chain works, not just the parts. This is the real
  Kaimi-pipeline E2E (distinct from the separate multi-agent orchestrator's tests).

**Done when:**
- **Contract layer (every commit):** full chain Hunter → Scorer → Manager → Outline → Writer → Final Review runs against mocked SAM.gov + mocked model + fixtures, asserting a valid `ready_to_submit` (or correctly-halted) result lands in the Store
- **E2E layer (run separately):** same chain against live SAM.gov + live Gemini, asserting structure and behavior (valid scored + drafted + reviewed Opportunity), not exact strings
- Covers at least: happy path, an ineligible opportunity dropped by Hunter, and a draft that fails Final Review
- Documented in the test suite how to run each layer

**Not this ticket:** load/stress testing, performance profiling.

---

### KAI-M7 — Google Drive/Docs integration foundation (Phase 3 dependency)
- **Phase:** 3 · **Zone:** 2 · **Owner:** Malik
- **Depends on:** GCP auth (done), Secret Manager (done)
- **Context:** KAI-5 (Outline → Google Doc, issue #5) is **blocked** waiting on this.
  Wire up Drive/Docs API access and auth so any agent can create a Doc and return a link.

**Done when:**
- A thin internal client can authenticate to Google Drive/Docs via the existing GCP service account / Secret Manager (no new secret-handling pattern unless justified + documented)
- Can create a Doc, write content, and return its URL; returns a clear error on failure (never loses content silently)
- Contract tests with a mocked Drive client; one live E2E creating-and-cleaning-up a real Doc behind a flag
- Security-sensitive (new external scope) — call out prominently in the PR

**Not this ticket:** the Outline→Doc agent logic itself (that's KAI-5 / #5, which this unblocks).

---

### KAI-M8 — Past-performance knowledge base (Phase 3, backlog)
- **Phase:** 3 · **Zone:** 1+2 · **Owner:** Malik · **Priority:** backlog (do not start before Gate 0 + KAI-M5/M6 land)
- **Context:** The Capability Profile (#9) intentionally holds only lightweight facts.
  The Scorer and Writer would both be stronger grounded in BlueMeta's full past
  performance. ARCHITECTURE.md scopes the rich knowledge base / RAG to Phase 3.

**Done when:**
- Design proposal (schema + retrieval approach) reviewed by Malik **before** any build — design eagerly, provision lazily
- Capability Profile can attach to it without breaking the existing struct (#9 designed for this)
- (Build acceptance criteria to be defined after the design is approved)

**Not this ticket:** anything until the design review is approved. This is a placeholder so it isn't forgotten.

---

## Build Sequence Summary

| Order | Ticket | Title | Phase | Owner | Blocked by |
|---|---|---|---|---|---|
| 0 | #8 | AgentResult contract (land it) | 0 | Malik | — |
| 0 | #7 | Final Review actual checks (land it) | 3 | Timm | #8 |
| 0 | — | PR hygiene + close #11 | — | team | — |
| 1 | KAI-M6 | Zone 1 scheduled pipeline runner | 1→3 | Malik | Hunter, Scorer |
| 2 | KAI-8 | Writer agent — skeleton | 3 | Timm | #8 |
| 3 | KAI-9 | Writer agent — real drafting | 3 | Timm | KAI-8 |
| 4 | KAI-M5 | Manager — Zone 2 orchestrator | 3 | Malik | #8, KAI-9, #7 |
| 5 | KAI-M7 | Google Drive/Docs foundation | 3 | Malik | — |
| 6 | #5 | Outline → Google Doc (unblocked by KAI-M7) | 3 | Timm | KAI-M7 |
| 7 | KAI-10 | End-to-end integration test | 3 | both | KAI-M5, KAI-M6, KAI-9, #7 |
| — | KAI-M8 | Past-performance knowledge base | 3 | Malik | design review (backlog) |

**End-to-end is "done" when:** KAI-M6 (Zone 1) + KAI-M5 (Zone 2) + KAI-10 (proof)
are merged — at that point a real SAM.gov opportunity flows all the way to a
human-reviewable, `ready_to_submit` draft.
