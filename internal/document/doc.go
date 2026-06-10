// Package document implements the Kaimi proposal document — the central
// artifact of the Zone 2 lifecycle (GitHub issue #154, epic #153).
//
// Per INTENT.md, the proposal is a living, structured document that agents
// and the human both work on: sections are data (never one opaque blob),
// every revision is attributed to its actor (an agent name or "human"),
// the version bumps on every actor handoff, and gap flags belong to the
// document. Both the web dashboard and the desktop app read and write
// proposals exclusively through this package, so the two clients can never
// fork the format.
//
// Storage layout, under the same base directory as the opportunity store:
//
//	<base>/proposals/<opportunityID>/document.json   canonical, structured
//	<base>/proposals/<opportunityID>/draft.md        human-readable mirror
//
// draft.md is rewritten on every save — the "auto-saved text file" the
// product promises — so the working draft is always openable outside the
// apps. document.json remains the source of truth.
package document
