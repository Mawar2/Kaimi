# Kaimi — Hackathon Demo Script
**Track 2: Optimize (Existing Agents)** · Target length: **≤ 3:00**

> [!TIP]
> Track 2 rewards *treating AI quality as a rigorous engineering discipline*. The through-line:
> **"a real, deployed federal-BD agent system that we hardened into measured, production-grade
> reliability."** Don't just show it work — show it *running on real data* and show the
> *measurement*. Hit all four judging axes: Technical (30%), Business (30%), Innovation (20%),
> Demo (20%).
>
> Before recording, replace every **[MALIK: …]** with a real artifact or number. Keep energy up;
> the system is real, so let that confidence show.

---

## 0:00 – 0:25 | The problem + who we are
**[Visual]** Kaimi wordmark, then the GitHub repo.
**[Script]** "I'm Malik, founder of BlueMeta Technologies. We chase federal contracts — and the
bottleneck is brutal: thousands of hours reading dense SAM.gov solicitations just to decide what
to bid, then drafting compliant proposals. **Kaimi is the autonomous BD pipeline I built to run
my own business**, using Google's Agent Development Kit and Gemini 2.5 Pro. For Track 2, we took
this working system and hardened it into something production-grade and *measured*."

## 0:25 – 1:00 | Zone 1 live — discovery & scoring
**[Visual]** Terminal: `go run ./cmd/pipeline --mode=live`. Show real scored output.
**[Script]** "Zone 1 is a scheduled batch pipeline — no orchestrator, state flows through a Store.
The **Hunter** pulls live SAM.gov opportunities and filters them against BlueMeta's real
capability profile — NAICS, set-asides — as a hard eligibility gate. The **Scorer** then uses
Gemini 2.5 Pro to read the solicitation and return a BID / NO-BID / REVIEW call **with explainable
reasoning**. This runs on a schedule in Cloud Run today."
**[On-screen]** [MALIK: a real opportunity — title + the Scorer's reasoning + score.]

## 1:00 – 1:40 | Zone 2 live — the grounded proposal + the human gate
**[Visual]** Select an opportunity; show the Manager threading the Zone-2 agents; show the draft.
**[Script]** "When I select an opportunity, Zone 2 spins up a **deterministic Go conductor — no LLM
in the orchestration** — that threads it through Outline, Writer, and Final Review. The Writer
**grounds strictly on our capability profile and the ingested solicitation documents** — it flags
gaps instead of fabricating, because we never promise the government something we can't deliver.
Final Review runs a compliance pass. Then it **stops at a human gate** — Kaimi never auto-submits.
A person approves every proposal."
**[On-screen]** [MALIK: a real drafted section + a flagged gap + the compliance result.]

## 1:40 – 2:20 | The Track-2 differentiator — reliability is MEASURED
**[Visual]** Terminal: run the eval harness; show the reliability report.
**[Script]** "Here's what makes this Track 2 and not a demo: we treat reliability as an engineering
discipline. Our **`internal/eval` harness** scores the Scorer's bid/no-bid accuracy against a
labeled dataset, and measures the Writer's **groundedness** — what fraction of every drafted claim
is actually supported by our source facts, flagging any fabrication. That turns 'it works' into a
*number we can defend and regression-test*."
**[On-screen]** [MALIK: real eval output — Scorer accuracy/precision/recall + Writer groundedness %.]

## 2:20 – 2:50 | Innovation — quality enforced by the system itself
**[Visual]** GitHub Actions: the AI Code Review + auto-fix on a PR.
**[Script]** "And quality is enforced automatically. Every pull request is reviewed by **Gemini 2.5
Pro in CI**, an auto-fix bot patches simple issues, and merges are gated on green checks. Combined
with the deterministic conductor and a crash-safe, status-driven human gate, Kaimi behaves like
infrastructure we'll operate for years — not a one-off."

## 2:50 – 3:00 | Close
**[Visual]** The dashboard showing a real opportunity moving through stages.
**[Script]** "Kaimi is a real, deployed, *measured* BD system that a one-person shop runs every day.
That's Track 2. Thank you."

---

### Pre-record checklist
- [ ] [MALIK: `gcloud auth application-default login` + `SAM_API_KEY` set so the live runs work on camera.]
- [ ] [MALIK: pick ONE real opportunity to follow through all the way — same one in Zone 1, Zone 2, and the dashboard.]
- [ ] [MALIK: run the eval harness once beforehand and have the report on screen.]
- [ ] Architecture diagram (ARCHITECTURE.md Mermaid / SystemDesign.jsx) ready as a cutaway.
- [ ] Record at 1080p; keep total under 3:00.
