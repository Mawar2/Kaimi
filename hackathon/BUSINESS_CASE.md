# Kaimi — Business Case (one-pager)
**Track 2: Optimize** · BlueMeta Technologies · 2026-06-10

> [MALIK: replace every [MALIK: …] with a real BlueMeta number before submission. Even rough,
> honest figures beat blanks — judges score the Business Case at 30%, and a *real* operator with
> *real* numbers is our biggest edge over teams with invented use-cases.]

## The problem
Federal contracting is a [MALIK: $X]-trillion market, but for a small business it's gated by
*labor*, not capability. Winning work means:
- Reading dense SAM.gov solicitations daily just to decide what's worth bidding.
- Drafting compliant, grounded proposals under deadline.
- Doing both with a one- or two-person team that can't afford a full capture department.

Today BlueMeta spends roughly **[MALIK: N hours/week]** triaging opportunities and
**[MALIK: M hours]** per proposal — time a small shop simply doesn't have.

## The solution — Kaimi
An autonomous, two-zone BD pipeline (Google ADK + Gemini 2.5 Pro) that BlueMeta runs in production:
- **Zone 1 (scheduled):** Hunter pulls live SAM.gov opportunities, filters against our real
  capability profile, and the Scorer returns an explainable BID/NO-BID call — so we only spend
  human attention on opportunities worth it.
- **Zone 2 (per proposal):** a deterministic conductor threads Outline → Writer → Final Review,
  drafting proposals **grounded strictly on our capabilities and the solicitation documents** —
  flagging gaps instead of fabricating — and **stopping at a human gate** before anything is sent.

## Quantified impact (BlueMeta)
| Metric | Before Kaimi | With Kaimi |
|--------|--------------|------------|
| Hours/week triaging opportunities | [MALIK: N] | [MALIK: N′] |
| Hours to first proposal draft | [MALIK: M] | [MALIK: M′] |
| Opportunities reviewed per week | [MALIK: …] | [MALIK: …] |
| Bid decisions backed by explainable scoring | Ad hoc | 100% |

**Bottom line:** [MALIK: one sentence — e.g. "Kaimi turns ~X hours of weekly capture work into
~Y, letting BlueMeta pursue Z× more qualified opportunities with the same headcount."]

## Why this is Track 2 (Optimize), not a prototype
We didn't just build agents — we hardened them into **measured, production-grade reliability**:
- An **`internal/eval` harness** scores Scorer accuracy and Writer groundedness, so reliability is
  a number we regression-test, not a hope.
- **AI code review (Gemini 2.5 Pro) gates every PR** in CI, with an auto-fix bot.
- A **deterministic, no-LLM conductor** and a **crash-safe, status-driven human gate** make the
  system debuggable, resumable, and safe to operate for years.

## Market beyond BlueMeta
Every small federal contractor has this exact pain. Kaimi is built on forward-compatible contracts
(the `Opportunity` schema, a swappable `Store`) so it generalizes — a credible path to a
[MALIK: brief market sizing / who else would pay for this] offering.
