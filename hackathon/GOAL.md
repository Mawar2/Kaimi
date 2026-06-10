# GOAL — Win the Google for Startups AI Agents Challenge (Track 2) with Kaimi

**Last updated:** 2026-06-10
**Owner:** Malik (malik@bluemetatech.com) — BlueMeta Technologies
**This file is the north star the autonomous build loop works against.** Read it in full at
the start of every loop iteration. When in doubt, the priority order in §4 wins.

---

## 1. The stakes (why this matters)

This is not a throwaway demo. Kaimi is **production infrastructure BlueMeta will operate
for years** to run its federal BD pipeline. The hackathon is a fast-track and a funding
event layered on top of a tool Malik already needs and will use.

- **Submission deadline:** 2026-06-11, 5:00 PM PT. Hard wall.
- **Prize pool:** $60K cash + $37.5K Cloud credits. Grand Prize $15K+$10K credits;
  **Best-of-Theme $10K+$7.5K credits (3 winners)**; Regional $5K+$2.5K credits (2).
- **Why we can win:** most entrants fabricate a business case. Malik *is* the user, on a
  *real* business, chasing *real* contracts. That authenticity is the moat.

## 2. Track & strategy

**Track 2 — Optimize (Existing Agents).** Chosen deliberately:
- Kaimi already exists and is deployed → it is not a Track-1 blank canvas.
- Track 2's thesis ("treat AI quality as a rigorous engineering discipline… stress-test
  multi-step reasoning… production-grade reliability") is *exactly* what Kaimi's two-layer
  testing, deterministic conductor, and forward-compatible contracts already embody.
- Track 1 is the most crowded room; Track 2 rewards the discipline we already have.
- "Best of Theme" is a winnable $10K + $7.5K-credits target on its own.

**The winning narrative:** "I took a working-in-a-sandbox federal-BD agent system and
hardened it into production-grade, reliable, *measured* infrastructure my one-person
business runs every day — and here is the eval harness that proves the reliability."

## 3. Judging criteria → what moves each number

| Criterion | Weight | What scores it for us |
|-----------|--------|------------------------|
| **Technical Implementation** | 30% | A working live end-to-end run; an **eval/optimization harness** (Track 2's signature ask); the open security fix (#129); the Zone-2 doc-ingestion chain completed; ADK + Gemini + MCP visibly used. |
| **Business Case** | 30% | A *real* BlueMeta opportunity flowing queue → scored → drafted proposal; quantified time saved per proposal; the solo-founder-runs-it-daily story. |
| **Innovation & Creativity** | 20% | Two-zone architecture; **deterministic Go conductor (no LLM)**; human-gate-as-status (crash-safe, resumable); AI-governance-as-product. |
| **Demo & Presentation** | 20% | A judge-reachable demo with login; a tight architecture diagram; a <3-min video that shows a real opportunity becoming a real draft. |

## 4. Prioritized backlog (the loop works top-down)

> Rule: **never skip a P0 to do a P1.** Submission gates beat polish. Each item becomes a
> GitHub issue with acceptance criteria before any code is written (see §5).

### P0 — Submission gates (cannot submit without these)
1. **Live end-to-end run works on one real opportunity.** Hunter → Scorer → Queue →
   Manager → Outline → Writer → Final Review, against a genuine SAM.gov opportunity, with
   the human-approval gate intact. This is the single most important deliverable for
   Track 2. (Confirms / closes the "pending first live run" risk.)
2. **Judge-reachable demo.** The web dashboard deployed at a stable URL, with login, that a
   judge can use to watch a real opportunity move through stages. Seed it with real data.
3. **Architecture diagram** — one clean visual of the two-zone system (export-ready).
4. **Demo video script + storyboard** (≤3 min): real opportunity → scored → drafted →
   human approves. Malik records; the loop produces the script, shot list, and the seeded
   demo state. *The loop does not record video.*

### P1 — Technical score (Track 2 reliability/optimization)  ✅ ALL DONE
5. ✅ **DONE (PR #181).** **#129 security fix** — redacted API key from SAM.gov client error messages.
6. ✅ **DONE (PR #183, `internal/eval`).** **Eval/optimization harness** — Scorer bid/no-bid
   accuracy + Writer groundedness, mock-tested in CI; the live reliability numbers need Malik's
   GCP creds (run `cmd/eval`). Track 2's signature deliverable.
7. ✅ **DONE (8-PR merge-train: #165/#184/#169/#170/#173/#175/#176/#187).** **Zone-2
   doc-ingestion chain** (#160→#161→#163→#164→#168→#171→#172) — solicitation documents now flow
   to a grounded, compliance-checked draft.
8. ✅ **DONE (PR #187).** **#174** — retired `manager.Manager`; `proposal.Service` is the single
   Zone-2 conductor.

### P2 — Polish / hardening (do only after P0+P1 are green)
9. #145 dashboard detail-page error/test hardening; #112/#113 QA handler + smoke tests;
   #114 dashboard security review; #115 dashboard usage docs.
10. Doc consistency sweep — single canonical model-id reference so model-name bumps stop
    rippling across every doc (the recurring `docs_standardize_model_id` churn).

### OUT of scope for the submission (do not build)
- Desktop dashboard epic (#136–#140) — separate epic, not needed to win. Leave it.
- #94 past-performance KB / Phase-4 RAG — leave a `// TODO(phase-4):` marker, do not build.

## 5. Loop operating rules (binding)

The loop honors Kaimi's existing contract (CLAUDE.md / WORKFLOW.md). Autonomy does **not**
lift the quality bar.

1. **Ticket gate.** No code without a GitHub issue with acceptance criteria + definition of
   done. If the next item has no ticket, the loop *creates* one (proposing AC) before
   building. Source of truth = GitHub Issues on `Mawar2/Kaimi`.
2. **TDD + two-layer tests.** Test first, watch it fail, then code. Unit/contract tests
   (mocked, fixtures) run every iteration; never hit live SAM.gov/Gemini in the fast layer.
3. **Branch per ticket** named per CONVENTIONS.md (`feature/NNN-…`). Commits
   `NNN_short_description`. One PR per ticket, referencing the issue.
4. **Merge guardrail (the one hard limit on autonomy).** The loop may merge its own PR
   **only when CI is fully green** — `make all` clean locally *and* GitHub CI (tests + lint
   + Gemini AI review) all passing. **Never force-merge. Never merge a red or pending PR.
   Never bypass CI or `--no-verify`.** If CI is red, fix it or stop and report; do not merge.
5. **Cost discipline (solo founder).** Prefer the free local/Gemini tiers for routine
   sub-work where quality allows (talk-to-gemma / talk-to-gemini via Antigravity) and
   reserve Claude/Gemini-Pro budget for the hard reasoning. Don't open PRs prematurely —
   each push triggers a ~$0.01 AI review.
6. **Anti-bloat.** Extend before creating. No `utils.go`/`helpers.go`. Justify any new file
   or dependency on its ticket. Keep `Opportunity` + `Store` forward-compatible.
7. **Stop-and-report conditions** (end the iteration, surface to Malik, do not improvise):
   - A live run needs credentials/secrets/GCP changes the loop can't safely make.
   - CI is red after a reasonable fix attempt.
   - A change touches IAM, Secret Manager, the `Opportunity` schema, or the `Store`
     interface in a security-sensitive way → flag for human eyes before merge.
   - Anything that would cost real money beyond credits, or submit a proposal to SAM.gov
     (never auto-submit — human approves).
8. **Every iteration ends with a one-line progress note**: what merged, what's next, any
   blocker. Keep this file's §4 checkboxes honest.

## 6. Definition of "won-ready"

We are submission-ready when ALL of these are true:
- [ ] P0 #1 — a real opportunity has flowed the full pipeline live, with evidence. **← MALIK (GCP creds)**
- [ ] P0 #2 — judge-reachable demo URL with login, seeded with real data. **← MALIK (deploy) / live-integration agent**
- [ ] P0 #3 — architecture diagram exported. **← MALIK (PR #146 / ARCHITECTURE.md)**
- [x] P0 #4 — **demo script drafted** (`hackathon/DEMO_SCRIPT.md`, Track-2, with `[MALIK]` placeholders); Malik records the video.
- [x] P1 #5 — #129 security leak fixed and merged (PR #181).
- [x] P1 #6 — eval harness built + unit-tested (PR #183); **live reliability numbers need Malik's GCP creds** (`cmd/eval`).
- [x] P1 #7 — Zone-2 doc-ingestion chain merged end-to-end (8 PRs).
- [x] P1 #8 — two Zone-2 orchestrators reconciled (PR #187).
- [ ] `make all` green on `main`; no secrets committed. **(green at each merge; re-confirm before submit)**
- [x] A one-page Business Case drafted (`hackathon/BUSINESS_CASE.md`); **Malik supplies the real numbers.**

### Status (2026-06-10) — the build is done; the rest is Malik-gated
The autonomous loop has landed the full Track-2 codebase: security fix, the `internal/eval`
reliability harness, and the entire Zone-2 ingestion epic (11 PRs total, all green-gated). The
demo script and business-case one-pager are drafted. **What remains needs Malik:** GCP creds for
the live end-to-end run (P0 #1) and demo deploy (P0 #2), the architecture diagram / docs (PR #146,
P0 #3), filling the `[MALIK]` placeholders, and recording the video. The parallel live-integration
agent (`hackathon/live-integration-agent.txt`) handles the live runtime + #162 GCS provisioning.

When the remaining boxes are checked, the loop **stops** and tells Malik it's time to record the
video and hit submit.
