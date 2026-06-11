# Devpost Submission — copy-paste pack
**Last updated:** 2026-06-11 · Deadline: 5:00 PM PT, June 11, 2026

Fill the Devpost form with the sections below. Items marked ⬜ need a value from Malik.

---

## Form fields

- **Project name:** Kaimi
- **Elevator pitch (tagline):**
  > Kaimi is an autonomous federal-BD pipeline in production at BlueMeta: it hunts SAM.gov
  > opportunities, makes explainable bid/no-bid calls with Gemini 2.5 Pro, and drafts grounded
  > proposals — hardened with a measured reliability harness and a human approval gate.
- **Theme (track):** Optimize (Existing Agents)
- **Region:** AMERS
- **Testing access:** Public repo — https://github.com/Mawar2/Kaimi — judges start at
  `hackathon/INSTRUCTIONS.md` (mock-data path requires no API keys)
- **Video:** ⬜ YouTube link
- **Repo link:** https://github.com/Mawar2/Kaimi

---

## Story (main description)

### Inspiration

BlueMeta Technologies is a small federal contractor. Like every small shop, we lose to the
calendar, not to competitors: reading dense SAM.gov solicitations daily just to decide what's
worth bidding, then drafting compliant proposals — all with a two-person team and no capture
department. We built Kaimi ("the seeker" in Hawaiian) because we needed it ourselves. It is
production infrastructure for our own daily business development, not a demo.

### What it does

Kaimi runs in two zones:

- **Zone 1 — scheduled pipeline (Cloud Run Job + Cloud Scheduler):** the **Hunter** agent pulls
  live SAM.gov opportunities by NAICS code and filters them through BlueMeta's real capability
  profile as a hard eligibility gate. The **Scorer** agent then reasons over each eligible
  solicitation with Gemini 2.5 Pro (via Vertex AI) and produces an *explainable* BID / NO_BID
  call — not just a number — into a shared opportunity queue.
- **Zone 2 — per-proposal lifecycle:** when a human selects an opportunity, a deterministic Go
  conductor threads it through **Outline → Technical Writer → Final Review**. The Writer drafts
  sections grounded strictly on our capability profile and the ingested solicitation documents
  (fetched and parsed into GCS via Document AI), and **flags gaps instead of fabricating**. Final
  Review runs a compliance pass. The flow always stops at a **human approval gate** — Kaimi never
  auto-submits.

A web dashboard and a desktop app (Wails) sit over the same data layer for queue review, gap
inspection, and the approval gate.

### Why Track 2: how we optimized it

Kaimi worked in a sandbox early. The work that earned this submission was making it survive the
real world — treating AI quality as a rigorous, *measured* engineering discipline:

- **Reliability as a number, not a hope.** The `internal/eval` harness scores the Scorer's
  bid/no-bid accuracy against a labeled dataset, and scores the Writer's **groundedness** — the
  fraction of drafted claims actually supported by source facts — flagging any fabrication. The
  metric logic is unit-tested in CI; the live reliability report runs against real APIs.
- **Two-layer testing.** Every package ships fast, deterministic unit/contract tests on cached
  fixtures (run on every commit), plus a separate live E2E layer against real SAM.gov and Gemini.
- **AI-gated CI/CD.** Every PR is reviewed by Gemini 2.5 Pro in GitHub Actions (bugs, security,
  Go idioms, architecture alignment), and an auto-fix bot applies the simple fixes automatically.
  Humans still merge every PR.
- **Edge cases found by real operation:** a NAICS filter fix that cut our SAM.gov API quota usage
  to ~3% of the daily limit; model fallback chains for Vertex AI availability; a crash-safe,
  status-driven human gate so a restart never loses a proposal mid-flight; gap flagging and
  highlighting so reviewers see exactly where the draft is thin instead of trusting it blindly.
- **A deterministic, no-LLM conductor.** Orchestration is plain Go: debuggable, resumable, and
  cheap. LLM reasoning is reserved for where it earns its cost (scoring, drafting, review).

### Quantified impact (estimated from early production operation)

| Metric | Before Kaimi | With Kaimi |
|--------|--------------|------------|
| Hours/week triaging opportunities | ~10 | ~1 |
| Hours to first proposal draft | ~30–40 | ~4–6 |
| Opportunities reviewed per week | ~25, manually | Every posting in our NAICS codes, scored |
| Bid decisions with explainable scoring | Ad hoc | 100% |

Roughly **5× more qualified bids with the same two-person team**. And every small federal
contractor — hundreds of thousands of small businesses are registered in SAM.gov — has this same
pain, which is the path beyond BlueMeta.

### How we built it

Go 1.25 throughout, chosen for legibility and operational simplicity. Gemini 2.5 Pro via
Vertex AI (`us-east4`) for all reasoning. Google Cloud for everything operational: Cloud Run
Jobs + Cloud Scheduler (Zone 1), GCS + Document AI (solicitation ingestion), Secret Manager +
service-account IAM (no keys in code), Google Docs/Drive export for finished drafts. The Google
ADK Go SDK anchors the platform foundation. Engineering discipline is enforced, not aspirational:
ticket-gated work, TDD, golangci-lint, and the Gemini-powered AI review on every PR.

### Challenges we ran into

- **LLM fabrication in proposal drafts** — solved by grounding the Writer strictly on the
  capability profile + ingested documents, then *measuring* groundedness in the eval harness.
- **SAM.gov's 1,000 req/day rate limit** — aggressive caching and a server-side NAICS filter fix
  dropped usage to ~3% of quota.
- **Human-in-the-loop without fragility** — the approval gate is status-driven and crash-safe;
  the orchestrator is deterministic Go, so a restart resumes instead of corrupting state.

### Accomplishments we're proud of

A real company runs this for real federal BD, today. Reliability is regression-tested as a
number. And the repo's CI reviews its own PRs with Gemini — the system that ships Kaimi is itself
an AI agent system.

### What's next

A cross-proposal knowledge base (RAG over past proposals and win/loss data), multi-tenancy on the
already-swappable `Store` contract, and a per-seat SaaS for small federal contractors.

---

## Built with (Devpost tags)

`go` · `gemini` · `vertex-ai` · `google-cloud` · `cloud-run` · `cloud-scheduler` ·
`cloud-storage` · `document-ai` · `secret-manager` · `github-actions` · `wails` · `sam-gov`

## Submission checklist

- ⬜ Upload video to YouTube, paste link above and in the Devpost video field
- ⬜ Devpost form: Theme = **Optimize (Existing Agents)**, Region = **AMERS**
- ⬜ Paste Story section into the description field
- ⬜ Repo link + note "judges start at `hackathon/INSTRUCTIONS.md`" in testing access
- ⬜ Architecture diagram: ARCHITECTURE.md (Mermaid) renders on GitHub; attach
  `hackathon/Kaimi Architecture.html` screenshot/export as the diagram image if the form wants an upload
- ⬜ Submit before **5:00 PM PT**
