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
	// TierPrimary indicates a primary NAICS code (strongest match).
	TierPrimary NAICSTier = "primary"

	// TierSecondary indicates a secondary NAICS code (moderate match).
	TierSecondary NAICSTier = "secondary"

	// TierTertiary indicates a tertiary NAICS code (weaker match).
	TierTertiary NAICSTier = "tertiary"
)

// NAICSCode represents a North American Industry Classification System code
// with its description and priority tier for weighted opportunity matching.
type NAICSCode struct {
	// Code is the 6-digit NAICS identifier (e.g., "541519").
	Code string `json:"code" yaml:"code"`

	// Description is the human-readable label for this code.
	Description string `json:"description" yaml:"description"`

	// Tier indicates the priority level (primary, secondary, or tertiary).
	Tier NAICSTier `json:"tier" yaml:"tier"`
}

// SetAsideStatus represents BlueMeta's eligibility for federal set-aside programs.
// Each field corresponds to a SAM.gov set-aside category.
type SetAsideStatus struct {
	// SmallBusiness indicates eligibility for small business set-asides (SBA/SBP).
	SmallBusiness bool `json:"small_business" yaml:"small_business"`

	// SDB indicates Small Disadvantaged Business certification.
	SDB bool `json:"sdb" yaml:"sdb"`

	// MinorityOwned indicates minority-owned business status (self-certified).
	MinorityOwned bool `json:"minority_owned" yaml:"minority_owned"`

	// EightA indicates 8(a) Business Development Program certification.
	EightA bool `json:"eight_a" yaml:"eight_a"`

	// SDVOSB indicates Service-Disabled Veteran-Owned Small Business certification.
	SDVOSB bool `json:"sdvosb" yaml:"sdvosb"`

	// WOSB indicates Women-Owned Small Business certification.
	WOSB bool `json:"wosb" yaml:"wosb"`

	// HUBZone indicates Historically Underutilized Business Zone certification.
	HUBZone bool `json:"hubzone" yaml:"hubzone"`

	// VOSB indicates Veteran-Owned Small Business certification.
	VOSB bool `json:"vosb" yaml:"vosb"`
}

// PastPerformance represents a past project or contract (lightweight fact record).
// Full narratives and embeddings will be added in Phase 3.
type PastPerformance struct {
	// Client is the client organization name.
	Client string `json:"client" yaml:"client"`

	// Scope is a brief description of the project.
	Scope string `json:"scope" yaml:"scope"`

	// Value is the contract value or engagement type.
	Value string `json:"value" yaml:"value"`

	// WhatItProves is a list of capabilities demonstrated by this project.
	WhatItProves []string `json:"what_it_proves" yaml:"what_it_proves"`
}

// CapabilityProfile holds BlueMeta Technologies' certifications, NAICS codes,
// and past performance for evaluating federal contracting opportunities.
//
// Hunter uses this profile to:
//   - Derive NAICS codes to search (via AllNAICSCodes)
//   - Gate out ineligible set-asides (via IsEligible)
//
// The profile can be loaded from a JSON or YAML file via LoadProfile, enabling
// updates without code changes.
type CapabilityProfile struct {
	// UEI is the Unique Entity Identifier (replaced DUNS in 2022).
	UEI string `json:"uei" yaml:"uei"`

	// CAGE is the Commercial and Government Entity code.
	CAGE string `json:"cage" yaml:"cage"`

	// Company is the legal company name.
	Company string `json:"company" yaml:"company"`

	// Address is the physical business address.
	Address string `json:"address" yaml:"address"`

	// NAICSCodes is the tiered list of codes BlueMeta can perform work under.
	// Hunter passes AllNAICSCodes() to the SAM.gov client.
	NAICSCodes []NAICSCode `json:"naics_codes" yaml:"naics_codes"`

	// SetAside indicates eligibility for federal set-aside programs.
	SetAside SetAsideStatus `json:"set_aside" yaml:"set_aside"`

	// Clearance is the security clearance level.
	Clearance string `json:"clearance" yaml:"clearance"`

	// Competencies is a list of core technical and domain competencies.
	Competencies []string `json:"competencies" yaml:"competencies"`

	// PastPerformance is a list of past projects/contracts.
	PastPerformance []PastPerformance `json:"past_performance" yaml:"past_performance"`
}

// AllNAICSCodes returns all NAICS codes as a flat string slice, preserving the
// tier-ordered sequence from the profile file.
//
// Hunter passes this list to the SAM.gov client to fetch opportunities.
func (p *CapabilityProfile) AllNAICSCodes() []string {
	codes := make([]string, 0, len(p.NAICSCodes))
	for _, n := range p.NAICSCodes {
		codes = append(codes, n.Code)
	}
	return codes
}

// IsEligible returns true if the opportunity passes BlueMeta's binary eligibility gate.
//
// The gate uses a known set-aside code switch. Unrecognized codes pass through
// to avoid false negatives — it is better to score an ineligible opportunity
// than to silently drop an eligible one.
func (p *CapabilityProfile) IsEligible(opp *opportunity.Opportunity) bool {
	return isEligibleCode(opp.SetAsideCode)
}

// isEligibleCode reports whether a SAM.gov set-aside code is eligible for BlueMeta.
// Normalizes to uppercase before matching. Unrecognized codes return true
// (conservative passthrough).
func isEligibleCode(setAsideCode string) bool {
	code := strings.ToUpper(strings.TrimSpace(setAsideCode))
	switch code {
	case "", "NONE":
		// Full-and-open: no set-aside restriction.
		return true
	case "SBA", "SBP":
		// Small business set-asides: BlueMeta qualifies as a small business.
		return true
	case "8A", "8(A)", "8AN":
		// 8(a) Business Development Program: cert not held.
		return false
	case "SDVOSB", "SDVOSBC", "SDVOSBS":
		// Service-Disabled Veteran-Owned Small Business: cert not held.
		return false
	case "WOSB", "EDWOSB", "WOSBSS", "EDWOSBSS":
		// Women-Owned Small Business: cert not held.
		return false
	case "HUBZONE", "HUB", "HZC", "HZS":
		// HUBZone: cert not held.
		return false
	case "VOSB":
		// Veteran-Owned Small Business: cert not held.
		return false
	case "IEE", "ISBEE":
		// Indian economic enterprise / Indian small business: cert not held.
		return false
	default:
		// Unknown codes pass through to avoid false negatives.
		return true
	}
}

// LoadProfile loads a CapabilityProfile from a JSON or YAML file.
// The file format is determined by the file extension (.json, .yaml, or .yml).
//
// Example:
//
//	p, err := profile.LoadProfile("profile.json")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	codes := p.AllNAICSCodes()
func LoadProfile(path string) (*CapabilityProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file: %w", err)
	}

	var p CapabilityProfile
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("failed to parse JSON profile: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("failed to parse YAML profile: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported profile file extension %q: use .json, .yaml, or .yml",
			filepath.Ext(path))
	}

	return &p, nil
}

// BlueMeta is the embedded default capability profile for BlueMeta Technologies.
// It is used as a fallback when no --profile flag is specified.
//
// For production use, load from profile.json at the repo root via LoadProfile.
var BlueMeta = &CapabilityProfile{
	UEI:     "XVUEA59LY579",
	CAGE:    "9RY40",
	Company: "BlueMeta Technologies",
	Address: "2 HOPKINS PLAZA, UNIT 1908, BALTIMORE, MD 21201-2946 USA",
	NAICSCodes: []NAICSCode{
		// Primary: strongest match
		{Code: "541519", Description: "Other Computer Related Services", Tier: TierPrimary},
		{Code: "541512", Description: "Computer Systems Design Services", Tier: TierPrimary},
		{Code: "541511", Description: "Custom Computer Programming Services", Tier: TierPrimary},
		// Secondary
		{Code: "518210", Description: "Computing Infrastructure Providers, Data Processing, Web Hosting, and Related Services", Tier: TierSecondary},
		{Code: "513210", Description: "Software Publishers", Tier: TierSecondary},
		{Code: "541715", Description: "Research and Development in the Physical, Engineering, and Life Sciences", Tier: TierSecondary},
		// Tertiary
		{Code: "541690", Description: "Other Scientific and Technical Consulting Services", Tier: TierTertiary},
		{Code: "561621", Description: "Security Systems Services (Except Locksmiths)", Tier: TierTertiary},
		{Code: "541513", Description: "Computer Facilities Management Services", Tier: TierTertiary},
	},
	SetAside: SetAsideStatus{
		SmallBusiness: true,
		SDB:           true,
		MinorityOwned: true,
	},
	Clearance: "Public Trust",
}
