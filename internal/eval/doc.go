// Package eval is the reliability harness for Kaimi's two LLM-backed agents:
// the Scorer (bid/no-bid accuracy) and the Writer (groundedness / no-fabrication).
//
// Its single responsibility is measuring agent reliability against a labeled
// dataset and producing a structured report. It does not score, draft, or call
// any model itself — it consumes a Scorer or section drafter through narrow
// consumer-side interfaces (Go idiom: accept interfaces) that the real
// scorer.GeminiScorer and writer.GeminiGenerator already satisfy. This keeps the
// harness decoupled from the agents and lets the fast test layer inject mocks.
//
// Two test layers, mirroring the rest of the repo:
//
//   - Fast layer (default `go test`): the metric math is proven against mock
//     agents with deterministic canned outputs. No network, no LLM. This is what
//     CI verifies.
//   - Live layer: cmd/eval wires the harness to the real Gemini-backed Scorer and
//     Writer and prints the report. It needs GCP auth and is never run in CI.
//
// Metrics:
//
//   - Scorer: accuracy, precision, and recall over the bid/no-bid labels, treating
//     BID as the positive class (see Report).
//   - Writer groundedness: the fraction of a draft's sentences whose significant
//     tokens are supported by the supplied facts, plus a must-not-fabricate check
//     (see CaseGroundedness for the heuristic and its documented limits).
//
// Ground truth: Malik is the BD authority. The seed dataset in
// test/fixtures/eval/ is a starting point he should validate and expand; the
// numbers this harness reports are only as good as those labels.
package eval
