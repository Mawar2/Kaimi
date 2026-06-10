// Package proposal implements the gated Zone 2 proposal lifecycle (GitHub
// issue #155, epic #153) — the shared service both the web dashboard and
// the desktop app call to wire the REAL agents into the product:
//
//	Select        → real Outline agent builds the document skeleton,
//	                real Technical Writer drafts every section, and the
//	                pipeline PAUSES at the single human review gate
//	UpdateSection → human edits at the gate become attributed revisions
//	Approve       → real Final Review agent runs on the document exactly
//	                as the human left it (human edits are first-class
//	                content, per INTENT.md); issues land as document flags
//	RequestChanges→ the draft returns to the Writer with the human's note
//	Submit        → always a human act; agents stand down
//
// The service composes the existing agents through the Manager's own
// interfaces (OutlineRunner, WriterRunner, Reviewer) without modifying
// them, persists ProposalStatus on every transition so polling UIs stay
// truthful, and stores all artifacts through internal/document.
package proposal
