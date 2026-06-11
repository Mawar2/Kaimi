package zone2view

import (
	"strings"
	"unicode"

	"github.com/Mawar2/Kaimi/internal/proposal"
)

// StageNames is the five-node Zone 2 pipeline vocabulary, in order.
var StageNames = [5]string{"Outline", "Technical Writer", "Human Review", "Final Review", "Submit"}

// View derives the pipeline position (0-4) and display state from the persisted
// ProposalStatus vocabulary. State is one of: "progress", "human", "done",
// "submitted", "failed". An unknown/empty status reads as early progress.
func View(status string) (stageIndex int, state string) {
	switch status {
	case proposal.StatusOutlineRunning:
		return 0, "progress"
	case proposal.StatusWriterRunning:
		return 1, "progress"
	case proposal.StatusGate, proposal.StatusReviewNeedsHuman:
		return 2, "human"
	case proposal.StatusReviewRunning:
		return 3, "progress"
	case proposal.StatusReadyToSubmit:
		return 4, "done"
	case proposal.StatusSubmitted:
		return 4, "submitted"
	case "outline:failed":
		return 0, "failed"
	case "writer:failed":
		return 1, "failed"
	case "final-review:failed":
		return 3, "failed"
	default:
		// Selected but no pipeline state yet (or a legacy status).
		return 0, "progress"
	}
}

// StatusPhrase is the named-teammate present-tense line for a proposal.
func StatusPhrase(stageIndex int, state string) string {
	switch state {
	case "human":
		return "Paused for your review"
	case "done":
		return "Ready to submit"
	case "submitted":
		return "Submitted"
	case "failed":
		return StageNames[stageIndex] + " hit a problem"
	}
	switch stageIndex {
	case 0:
		return "Noa outlining now"
	case 1:
		return "Tomás drafting now"
	case 3:
		return "Vera finalizing"
	}
	return StageNames[stageIndex] + " in progress"
}

// Criterion is one must-have requirement checked against the current draft. It
// is the shared view-model for the gate's criteria grid on both web and desktop.
type Criterion struct {
	Label string `json:"label"`
	Note  string `json:"note"`
	OK    bool   `json:"ok"`
}

// DeriveCriteria checks each requirement against the draft (which must already be
// lowercased) and the Final Review's open flags. The open flags are
// authoritative: a must-have the review flagged as not addressed shows red here
// too, so the gate's criteria grid can never contradict the review's findings
// (tester-reported follow-up to #246 B6 — the lenient term-overlap matcher was
// showing green for must-haves the review flagged as not addressed). When a
// requirement is not flagged, the matcher provides the pre-review signal and an
// unconfirmed item carries honest copy. openFlags are the texts of the
// document's unresolved flags. Returns nil for no requirements.
func DeriveCriteria(requirements []string, draftLower string, openFlags []string) []Criterion {
	if len(requirements) == 0 {
		return nil
	}
	flagsLower := make([]string, len(openFlags))
	for i, f := range openFlags {
		flagsLower[i] = strings.ToLower(f)
	}
	items := make([]Criterion, 0, len(requirements))
	for _, req := range requirements {
		c := Criterion{Label: req}
		if flaggedAsUnaddressed(flagsLower, strings.ToLower(req)) {
			c.OK = false
			c.Note = "Final review flagged this as not yet addressed in the draft"
		} else {
			c.OK = RequirementAddressed(draftLower, req)
			if !c.OK {
				c.Note = "Kaimi could not auto-confirm this — verify in the draft"
			}
		}
		items = append(items, c)
	}
	return items
}

// flaggedAsUnaddressed reports whether any open Final Review flag names the
// requirement. The deterministic must_have flag embeds the requirement text
// verbatim, so a substring match is sufficient.
func flaggedAsUnaddressed(flagsLower []string, reqLower string) bool {
	if reqLower == "" {
		return false
	}
	for _, f := range flagsLower {
		if strings.Contains(f, reqLower) {
			return true
		}
	}
	return false
}

// requirementStopwords are common words dropped before term matching so the
// signal comes from the requirement's meaningful terms, not filler.
var requirementStopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "must": true,
	"shall": true, "will": true, "have": true, "from": true, "that": true,
	"this": true, "any": true, "all": true, "are": true, "per": true,
	"including": true, "provide": true, "required": true,
}

// RequirementAddressed reports whether the draft plausibly addresses a
// requirement. A verbatim full-phrase match falsely flags paraphrases as
// missing; instead this scores the overlap of the requirement's significant
// terms against the draft, comparing stems so "authorization" is satisfied by
// "authorized". A requirement counts as addressed when at least two-thirds of
// its significant terms appear — lenient enough to tolerate a synonym swap,
// strict enough not to match on noise. The draft must already be lowercased.
func RequirementAddressed(draftLower, requirement string) bool {
	terms := significantRequirementTerms(requirement)
	if len(terms) == 0 {
		// No meaningful terms (e.g. a requirement of only stopwords): fall back
		// to the whole-phrase check rather than claiming a spurious match.
		return strings.Contains(draftLower, strings.ToLower(strings.TrimSpace(requirement)))
	}
	hits := 0
	for _, term := range terms {
		if strings.Contains(draftLower, stemTerm(term)) {
			hits++
		}
	}
	return hits*3 >= len(terms)*2
}

// significantRequirementTerms splits a requirement into lowercased, meaningful
// terms: alphanumeric runs of length >= 3 that are not stopwords.
func significantRequirementTerms(requirement string) []string {
	var terms []string
	for _, field := range strings.FieldsFunc(strings.ToLower(requirement), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		if len(field) < 3 || requirementStopwords[field] {
			continue
		}
		terms = append(terms, field)
	}
	return terms
}

// stemTerm trims a few common English suffixes so inflected forms match a shared
// stem ("authorization"/"authorized" -> "author", "modernization"/"modernize"
// -> "modern"). Longest suffixes are checked first; the length guard keeps short
// words intact.
func stemTerm(term string) string {
	for _, suf := range []string{"ization", "isation", "ation", "izing", "ized", "izes", "ing", "ed", "es", "s"} {
		if len(term) > len(suf)+2 && strings.HasSuffix(term, suf) {
			return term[:len(term)-len(suf)]
		}
	}
	return term
}
