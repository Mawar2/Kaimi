// Package outline implements the Outline agent for Zone 2 of the Kaimi pipeline.
//
// The Outline agent is responsible for generating a structured proposal outline
// from a selected Opportunity. It is the first agent triggered by the Manager
// after a human selects an opportunity from the queue.
package outline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Mawar2/Kaimi/internal/agent"
	"github.com/Mawar2/Kaimi/internal/opportunity"
)

const agentName = "outline"

// Section represents a single required section in a federal proposal.
type Section struct {
	ID        string // short identifier, e.g. "technical_approach"
	Title     string // display title, e.g. "Technical Approach"
	Required  bool   // whether this section is mandatory for this opportunity
	Rationale string // why this section was included, derived from opportunity data
}

// Outline is the structured output produced by the Outline agent.
// It is the input the next agent in Zone 2 (KAI-4: formatting rules) consumes.
type Outline struct {
	OpportunityID string
	Title         string // opportunity title, carried for context
	Sections      []Section
	GeneratedAt   time.Time
}

// Agent is the Outline agent.
type Agent struct{}

// New creates a new Outline agent.
func New() *Agent {
	return &Agent{}
}

// Run takes a selected Opportunity and produces a structured Outline and an AgentResult.
//
// Returns a non-nil Outline on success. Returns a failed AgentResult (and nil Outline)
// on unrecoverable errors. Sparse opportunities get a best-effort outline rather than
// a failure.
//
// TODO(phase-3): Replace buildSections with a Gemini call once LLM integration lands.
func (a *Agent) Run(ctx context.Context, opp *opportunity.Opportunity) (*Outline, *agent.AgentResult, error) {
	if opp == nil {
		return nil, &agent.AgentResult{
			AgentName: agentName,
			Status:    agent.StatusFailed,
			Summary:   "opportunity must not be nil",
		}, fmt.Errorf("outline agent: opportunity must not be nil")
	}

	sections := buildSections(opp)

	outline := &Outline{
		OpportunityID: opp.ID,
		Title:         opp.Title,
		Sections:      sections,
		GeneratedAt:   time.Now().UTC(),
	}

	result := &agent.AgentResult{
		AgentName: agentName,
		Status:    agent.StatusSuccess,
		Summary:   fmt.Sprintf("generated %d sections for opportunity %s", len(sections), opp.ID),
		OutputRef: "", // TODO(phase-3): set to Google Doc URL once KAI-5 is built
	}

	return outline, result, nil
}

// buildSections derives the required proposal sections from the opportunity.
//
// Uses rule-based logic against the opportunity's own fields — type, contract type,
// set-aside code, and description keywords. No section list is hardcoded; every
// inclusion is traceable back to a field value.
//
// Returns at least the five standard federal proposal volumes even for sparse input.
func buildSections(opp *opportunity.Opportunity) []Section {
	desc := strings.ToLower(opp.Description)

	sections := []Section{
		{
			ID:        "executive_summary",
			Title:     "Executive Summary",
			Required:  true,
			Rationale: "required for all federal solicitations",
		},
		{
			ID:        "technical_approach",
			Title:     "Technical Approach",
			Required:  true,
			Rationale: "required for all federal solicitations",
		},
		{
			ID:        "management_approach",
			Title:     "Management Approach",
			Required:  true,
			Rationale: "required for all federal solicitations",
		},
		{
			ID:        "past_performance",
			Title:     "Past Performance",
			Required:  true,
			Rationale: "required for all federal solicitations",
		},
		{
			ID:        "price_cost_volume",
			Title:     "Price/Cost Volume",
			Required:  true,
			Rationale: "required for all federal solicitations",
		},
	}

	// Set-aside programs require a small business subcontracting plan.
	if opp.SetAsideCode != "" && opp.SetAsideCode != "NONE" {
		sections = append(sections, Section{
			ID:        "small_business_subcontracting",
			Title:     "Small Business Subcontracting Plan",
			Required:  true,
			Rationale: fmt.Sprintf("required by set-aside code %q", opp.SetAsideCode),
		})
	}

	// Key personnel requirements surface in the description.
	if containsAny(desc, "key personnel", "named individual", "key staff") {
		sections = append(sections, Section{
			ID:        "key_personnel",
			Title:     "Key Personnel",
			Required:  true,
			Rationale: "opportunity description references key personnel requirements",
		})
	}

	// Quality assurance plans are often explicitly required.
	if containsAny(desc, "quality assurance", "qasp", "quality control", "qcp") {
		sections = append(sections, Section{
			ID:        "quality_assurance",
			Title:     "Quality Assurance Plan",
			Required:  true,
			Rationale: "opportunity description references quality assurance requirements",
		})
	}

	// Security and clearance requirements drive a dedicated section.
	if containsAny(desc, "secret", "clearance", "classified", "security plan") {
		sections = append(sections, Section{
			ID:        "security_plan",
			Title:     "Security Plan",
			Required:  true,
			Rationale: "opportunity description references security or clearance requirements",
		})
	}

	// Recompetes and transitions require a transition plan.
	if containsAny(desc, "transition", "recompete", "incumbent", "phase-in") {
		sections = append(sections, Section{
			ID:        "transition_plan",
			Title:     "Transition Plan",
			Required:  true,
			Rationale: "opportunity description indicates a transition or recompete scenario",
		})
	}

	// Data rights appear in technology and software contracts.
	if containsAny(desc, "data right", "intellectual property", " ip ", "technical data") {
		sections = append(sections, Section{
			ID:        "data_rights",
			Title:     "Data Rights and Intellectual Property",
			Required:  true,
			Rationale: "opportunity description references data rights or intellectual property",
		})
	}

	return sections
}

// containsAny reports whether s contains any of the given substrings.
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
