// Package scorer implements the Scorer agent for the Kaimi BD pipeline.
//
// The Scorer takes an eligible Opportunity and the company's Capability Profile,
// then produces a 0-100 fit score, a BID/NO_BID/REVIEW recommendation, and
// human-readable reasoning explaining the decision.
//
// The scoring process:
//  1. Pre-computes deterministic signals: NAICS match (primary > secondary),
//     competency tag overlap, past performance relevance, SDB applicability.
//  2. Sends the opportunity text, profile, and pre-computed signals to Gemini
//     via Vertex AI to synthesize a score with explained reasoning.
//  3. Validates and converts the structured LLM response to a ScoreResult.
//  4. Writes the enriched Opportunity back to the Store via ScoreAndSave.
//
// Zone: Zone 1 (scheduled pipeline). The Scorer runs after the Hunter in the
// daily batch job: Hunter → Scorer → Opportunity Queue.
package scorer
