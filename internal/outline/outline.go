// Package outline defines the output types for the Outline agent and the
// logic for building a proposal section structure from a SAM.gov opportunity.
//
// The Outline agent is a Zone 2 agent (Phase 3). It takes an enriched
// Opportunity (scored by the Scorer in Phase 1) and produces an Outline —
// an ordered list of Sections — for the Writer agent (KAI-4) to expand into
// proposal prose.
//
// buildSections() derives sections from the opportunity rather than using a
// hardcoded list: five standard federal proposal volumes are always included,
// and conditional sections are added based on the set-aside code and keywords
// found in the opportunity description.
package outline

import (
	"fmt"
	"strings"
	"time"

	"github.com/Mawar2/Kaimi/internal/opportunity"
)

// Section represents one volume or section of a federal proposal outline.
//
// Every section has a Rationale that records why it was included — either
// because it is a required baseline volume, or because the opportunity's
// set-aside code or description keywords triggered it. The rationale is
// consumed by the Writer agent as context for how to frame that section.
type Section struct {
	Title     string // Section title (e.g., "Technical Approach")
	Rationale string // Why this section was included in the outline
}

// Outline is the structured output of the Outline agent.
//
// It contains an ordered list of Sections derived from the opportunity.
// The Writer agent (KAI-4) consumes Outline as input to expand each section
// into proposal prose.
type Outline struct {
	OpportunityID string    // References the source Opportunity by ID
	Sections      []Section // Ordered list of proposal sections
	GeneratedAt   time.Time // When this outline was produced
}

// buildSections derives the proposal sections required for the given opportunity.
//
// The five standard federal proposal volumes are always included as a baseline
// even for sparse opportunities with minimal metadata:
//
//  1. Executive Summary
//  2. Technical Approach
//  3. Management Approach
//  4. Past Performance
//  5. Price/Cost
//
// Conditional sections are then appended based on:
//   - SetAsideCode (non-empty): Subcontracting Plan
//   - Description keywords: Key Personnel, Quality Assurance, Security Plan,
//     Transition Plan, Data Rights
//
// Keyword matching is case-insensitive.
func buildSections(opp *opportunity.Opportunity) []Section {
	sections := []Section{
		{
			Title:     "Executive Summary",
			Rationale: "Required baseline: establishes offeror identity and value proposition",
		},
		{
			Title:     "Technical Approach",
			Rationale: "Required baseline: demonstrates technical understanding and proposed solution",
		},
		{
			Title:     "Management Approach",
			Rationale: "Required baseline: describes team structure and project management methodology",
		},
		{
			Title:     "Past Performance",
			Rationale: "Required baseline: provides evidence of relevant prior contract experience",
		},
		{
			Title:     "Price/Cost",
			Rationale: "Required baseline: defines pricing structure and cost basis",
		},
	}

	// A non-empty set-aside code means the agency has reserved this contract
	// for a specific small business category. FAR 19.7 requires a subcontracting
	// plan in that case.
	if opp.SetAsideCode != "" {
		sections = append(sections, Section{
			Title:     "Subcontracting Plan",
			Rationale: fmt.Sprintf("Triggered by set-aside code %q: documents small business subcontracting commitments per FAR 19.7", opp.SetAsideCode),
		})
	}

	desc := strings.ToLower(opp.Description)

	if strings.Contains(desc, "key personnel") || strings.Contains(desc, "key staff") {
		sections = append(sections, Section{
			Title:     "Key Personnel",
			Rationale: "Triggered by keyword in description: identifies individuals critical to contract performance",
		})
	}

	if strings.Contains(desc, "quality assurance") {
		sections = append(sections, Section{
			Title:     "Quality Assurance",
			Rationale: "Triggered by keyword in description: defines quality control approach and metrics",
		})
	}

	if strings.Contains(desc, "security") || strings.Contains(desc, "clearance") {
		sections = append(sections, Section{
			Title:     "Security Plan",
			Rationale: "Triggered by keyword in description: addresses security requirements and clearance approach",
		})
	}

	if strings.Contains(desc, "transition") || strings.Contains(desc, "phase-in") || strings.Contains(desc, "phase in") {
		sections = append(sections, Section{
			Title:     "Transition Plan",
			Rationale: "Triggered by keyword in description: describes incumbent transition and knowledge transfer strategy",
		})
	}

	if strings.Contains(desc, "data rights") || strings.Contains(desc, "intellectual property") {
		sections = append(sections, Section{
			Title:     "Data Rights",
			Rationale: "Triggered by keyword in description: addresses data rights, IP provisions, and DFARS clauses",
		})
	}

	return sections
}
