# Kaimi — Business Case (one-pager)
**Track 2: Optimize** · BlueMeta Technologies · 2026-06-11

## The problem
Federal contracting is a **$750B+ annual market**, but for a small business it's gated by
*labor*, not capability. Winning work means:
- Reading dense SAM.gov solicitations daily just to decide what's worth bidding.
- Drafting compliant, grounded proposals under deadline.
- Doing both with a one- or two-person team that can't afford a full capture department.

Today BlueMeta spends roughly **10 hours/week** triaging opportunities and on the order of
**30–40 hours** per proposal draft — time a small shop simply doesn't have.

## The solution — Kaimi
An autonomous, two-zone BD pipeline (Google ADK + Gemini 2.5 Pro) that BlueMeta runs in production:
- **Zone 1 (scheduled):** Hunter pulls live SAM.gov opportunities, filters against our real
  capability profile, and the Scorer returns an explainable BID/NO-BID call — so we only spend
  human attention on opportunities worth it.
- **Zone 2 (per proposal):** a deterministic conductor threads Outline → Writer → Final Review,
  drafting proposals **grounded strictly on our capabilities and the solicitation documents** —
  flagging gaps instead of fabricating — and **stopping at a human gate** before anything is sent.

## Quantified impact (BlueMeta, estimated from early production operation)
| Metric | Before Kaimi | With Kaimi |
|--------|--------------|------------|
| Hours/week triaging opportunities | ~10 | ~1 (reviewing the scored queue) |
| Hours to first proposal draft | ~30–40 | ~4–6 (review + edit of a grounded draft) |
| Opportunities reviewed per week | ~25, manually | Every new posting in our NAICS codes, scored |
| Bid decisions backed by explainable scoring | Ad hoc | 100% |

**Bottom line:** Kaimi turns ~10 hours of weekly triage into ~1 and cuts a first proposal draft
from ~35 hours to ~5 — letting BlueMeta evaluate every opportunity in its NAICS codes and pursue
roughly **5× more qualified bids with the same two-person team**.

## Why this is Track 2 (Optimize), not a prototype
We didn't just build agents — we hardened them into **measured, production-grade reliability**:
- An **`internal/eval` harness** scores Scorer accuracy and Writer groundedness, so reliability is
  a number we regression-test, not a hope.
- **AI code review (Gemini 2.5 Pro) gates every PR** in CI, with an auto-fix bot.
- A **deterministic, no-LLM conductor** and a **crash-safe, status-driven human gate** make the
  system debuggable, resumable, and safe to operate for years.

## Market beyond BlueMeta
Every small federal contractor has this exact pain: **hundreds of thousands of small businesses
are registered in SAM.gov**, and tens of thousands actively compete for set-aside contracts with
no capture department. Kaimi is built on forward-compatible contracts (the `Opportunity` schema,
a swappable `Store`) so it generalizes — a credible path to a per-seat SaaS for small federal
contractors, where even a $200/month tool replacing 9 hours/week of capture labor pays for itself
in the first day of each month.
