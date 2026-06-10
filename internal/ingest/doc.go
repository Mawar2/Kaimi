// Package ingest is the Zone 2 document-ingestion stage (KAI / issue #161).
//
// Given an eligible Opportunity, the Manager runs ingestion as its first step
// (issue #163): for each SAM.gov attachment URL the stage fetches the file,
// stores the raw bytes in GCS (so a human can re-download the original), extracts
// the document's text, stores that text in GCS, and returns one
// opportunity.SolicitationDoc per attachment. Downstream agents (Outline, Writer,
// Final Review) then ground on the real document text instead of the thin SAM.gov
// summary alone.
//
// The stage is deterministic — no LLM. Its three collaborators are interfaces so
// the Manager and tests can inject fakes and so the production wiring (HTTP,
// Google Cloud Storage, Google Document AI) stays out of the orchestration logic:
//
//   - Fetcher     retrieves an attachment from its URL.
//   - ObjectStore persists raw bytes and extracted text (GCS in production).
//   - Extractor   turns raw document bytes into plain text (Document AI in
//     production; see documentai.go and docx.go).
//
// Anti-fabrication: a document whose text cannot be extracted (e.g. a scanned PDF
// with no embedded text) is never dropped silently and never invented. Its raw
// file is still saved for re-download, the SolicitationDoc is recorded with an
// empty TextObject, and the run is routed to needs_human so a person can look.
package ingest
