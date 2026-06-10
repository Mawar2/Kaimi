# Eval seed dataset

**Last updated:** 2026-06-10

Seed labeled cases for the reliability harness in `internal/eval` (GitHub issue #182).

- `scorer_cases.json` — bid/no-bid cases. Each references an `opp_*.json` Opportunity
  fixture in this directory and carries the human-labeled `expected_recommendation`
  (`BID` / `NO_BID` / `REVIEW`) plus optional `expected_reason_keywords`.
- `writer_cases.json` — Writer groundedness cases. Each carries a `section_prompt`, the
  `facts` the section is allowed to assert, and `must_not_fabricate` terms that signal
  fabrication if they appear in the draft.
- `opp_*.json` — Opportunity fixtures in the `internal/opportunity` JSON schema, reused
  by the scorer cases.

## Ground truth is Malik

These labels are a **starting point**, not authoritative truth. Malik is the BD ground
truth: the `expected_recommendation` labels and `facts`/`must_not_fabricate` lists should
be reviewed and expanded by him before any reliability number from this harness is
trusted or reported. The harness is only as good as these labels.

The capability profile used for scoring lives at `test/fixtures/capability_profile.json`.
