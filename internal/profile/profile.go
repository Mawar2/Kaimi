// Package profile defines BlueMeta Technologies' capability profile for evaluating
// federal contracting opportunities.
//
// The CapabilityProfile encodes what BlueMeta is legally eligible to bid on.
// Hunter uses this profile to gate out set-asides for programs BlueMeta does not
// hold, before opportunities reach the Scorer.
package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Mawar2/Kaimi/internal/opportunity"
)

// NAICSTier represents the priority tier of a NAICS code.
// Primary codes are the strongest match, followed by Secondary and Tertiary.
type NAICSTier string

const (
	// TierPrimary indicates a primary NAICS code (core competency)
	TierPrimary NAICSTier = "primary"
	// TierSecondary indicates a secondary NAICS code (moderate match)
	TierSecondary NAICSTier = "secondary"
	// TierTertiary indicates a tertiary NAICS code (adjacent capability)
	TierTertiary NAICSTier = "tertiary"
)

// NAICSCode represents a North American Industry Classification System code with tier priority.
type NAICSCode struct {
	Code        string    `yaml:"code"        json:"code"`
	Description string    `yaml:"description" json:"description"`
	Tier        NAICSTier `yaml:"tier"        json:"tier"`
}

// SetAsideStatus records which federal set-aside certifications BlueMeta holds.
// Used by the Scorer for fit reasoning; Hunter uses IsEligible() for hard gating.
type SetAsideStatus struct {
	SmallBusiness bool `yaml:"small_business" json:"small_business"`
	SDB           bool `yaml:"sdb"            json:"sdb"`
	MinorityOwned bool `yaml:"minority_owned" json:"minority_owned"`
	EightA        bool `yaml:"eight_a"        json:"eight_a"`
	SDVOSB        bool `yaml:"sdvosb"         json:"sdvosb"`
	WOSB          bool `yaml:"wosb"           json:"wosb"`
	HUBZone       bool `yaml:"hubzone"        json:"hubzone"`
}

// PastPerformance is a lightweight past-project record used by the Scorer for fit reasoning.
// Full narratives and embeddings will be added in Phase 3.
type PastPerformance struct {
	Client       string   `yaml:"client"         json:"client"`
	Scope        string   `yaml:"scope"          json:"scope"`
	Value        string   `yaml:"value"          json:"value"`
	WhatItProves []string `yaml:"what_it_proves" json:"what_it_proves"`
}

// CapabilityProfile holds BlueMeta Technologies' certifications, NAICS codes, and past
// performance. Hunter uses it for eligibility gating; Scorer uses it for fit reasoning.
type CapabilityProfile struct {
	UEI             string            `yaml:"uei"              json:"uei"`
	CAGE            string            `yaml:"cage"             json:"cage"`
	Company         string            `yaml:"company"          json:"company"`
	Address         string            `yaml:"address"          json:"address"`
	NAICSCodes      []NAICSCode       `yaml:"naics_codes"      json:"naics_codes"`
	SetAside        SetAsideStatus    `yaml:"set_aside"        json:"set_aside"`
	Clearance       string            `yaml:"clearance"        json:"clearance"`
	Competencies    []string          `yaml:"competencies"     json:"competencies"`
	PastPerformance []PastPerformance `yaml:"past_performance" json:"past_performance"`
}

// LoadProfile reads a CapabilityProfile from path, selecting the JSON or YAML parser
// by file extension (.json → JSON, .yaml/.yml → YAML).
func LoadProfile(path string) (*CapabilityProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file: %w", err)
	}

	var p CapabilityProfile
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("failed to parse profile JSON: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("failed to parse profile YAML: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported profile file extension: %s", filepath.Ext(path))
	}

	return &p, nil
}

// AllNAICSCodes returns a flat slice of code strings from all tiers in declaration order.
// Hunter uses this to query SAM.gov when no NAICS override is specified.
func (p *CapabilityProfile) AllNAICSCodes() []string {
	codes := make([]string, 0, len(p.NAICSCodes))
	for _, nc := range p.NAICSCodes {
		codes = append(codes, nc.Code)
	}
	return codes
}

// IsEligible returns true if the opportunity passes BlueMeta's binary eligibility gate.
//
// Decisions are driven by a switch over known SAM.gov set-aside codes. Unrecognized
// codes pass through as eligible to avoid false negatives (conservative default).
//
// Decision table:
//
//	""/"NONE"            → eligible  (full-and-open)
//	SBA/SBP/SDB         → eligible  (small business set-asides)
//	8A/8(A)/8AN         → ineligible (8(a) cert not held)
//	SDVOSB/SDVOSBC      → ineligible (SDVOSB cert not held)
//	WOSB/EDWOSB         → ineligible (women-owned cert not held)
//	HUBZONE/HUB         → ineligible (HUBZone cert not held)
//	VOSB                → ineligible (veteran-owned cert not held)
//	IEE/ISBEE           → ineligible (Indian enterprise cert not held)
//	unrecognized        → eligible  (conservative passthrough)
func (p *CapabilityProfile) IsEligible(opp *opportunity.Opportunity) bool {
	code := strings.ToUpper(strings.TrimSpace(opp.SetAsideCode))
	switch code {
	case "", "NONE":
		// Full-and-open competition: no restriction
		return true
	case "SBA", "SBP", "SDB":
		// Small business / small disadvantaged business: BlueMeta qualifies
		return true
	case "8A", "8(A)", "8AN":
		// 8(a) Business Development Program: BlueMeta not certified
		return false
	case "SDVOSB", "SDVOSBC", "SDVOSBS":
		// Service-Disabled Veteran-Owned Small Business: not held
		// SDVOSBS is the legacy SAM.gov sole-source code
		return false
	case "WOSB", "WOSBSS", "EDWOSB", "EDWOSBSS":
		// Women-Owned / Economically Disadvantaged Women-Owned: not held
		// WOSBSS/EDWOSBSS are legacy SAM.gov sole-source codes
		return false
	case "HUBZONE", "HUB", "HZC", "HZS":
		// HUBZone: not held
		// HZC/HZS are legacy SAM.gov set-aside and sole-source codes
		return false
	case "VOSB":
		// Veteran-Owned Small Business (non-service-disabled): not held
		return false
	case "IEE", "ISBEE":
		// Indian Economic Enterprise / Indian Small Business Economic Enterprise: not held
		return false
	default:
		// Unrecognized code: pass through to avoid starving the pipeline
		return true
	}
}
