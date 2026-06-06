// Package profile provides types and loading utilities for BlueMeta Technologies'
// federal contracting capability profile.
//
// The CapabilityProfile drives two concerns in the Hunter agent:
//   - NAICS code selection: AllNAICSCodes returns all codes across tiers for SAM.gov queries.
//   - Set-aside eligibility: the isEligible function in cmd/hunter uses the profile's
//     set_aside flags to decide which competitions BlueMeta can enter.
//
// Profiles are stored as JSON or YAML files (see LoadProfile). The canonical
// project profile is profile.json at the repository root.
package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// CapabilityProfile describes BlueMeta Technologies' federal contracting capabilities.
// It drives NAICS code search and set-aside eligibility gating in the Hunter agent.
type CapabilityProfile struct {
	Name            string            `json:"name"             yaml:"name"`
	SetAside        SetAsideStatus    `json:"set_aside"        yaml:"set_aside"`
	NAICS           NAICSTiers        `json:"naics"            yaml:"naics"`
	PastPerformance []PastPerformance `json:"past_performance" yaml:"past_performance"`
}

// SetAsideStatus records which small-business certifications BlueMeta holds.
// These flags determine eligibility for restricted competitions.
type SetAsideStatus struct {
	SmallBusiness bool `json:"small_business" yaml:"small_business"`
	SDB           bool `json:"sdb"            yaml:"sdb"` // Small Disadvantaged Business
	MinorityOwned bool `json:"minority_owned" yaml:"minority_owned"`
}

// NAICSTiers organizes NAICS codes by relevance priority.
// All three tiers are searched by Hunter; primary is BlueMeta's core competency.
type NAICSTiers struct {
	Primary   string   `json:"primary"   yaml:"primary"`
	Secondary []string `json:"secondary" yaml:"secondary"`
	Tertiary  []string `json:"tertiary"  yaml:"tertiary"`
}

// PastPerformance represents one completed federal contract, used by downstream
// scoring and proposal agents to match relevant experience to opportunities.
type PastPerformance struct {
	Title     string  `json:"title"     yaml:"title"`
	Customer  string  `json:"customer"  yaml:"customer"`
	NAICSCode string  `json:"naics"     yaml:"naics"`
	Value     float64 `json:"value"     yaml:"value"`     // contract value in USD
	Year      int     `json:"year"      yaml:"year"`      // year of completion
	Relevance string  `json:"relevance" yaml:"relevance"` // "high", "medium", or "low"
}

// AllNAICSCodes returns all NAICS codes across tiers, deduplicated, with primary
// first followed by secondary then tertiary. The order matters: SAM.gov results
// for higher-priority codes are processed first by the eligibility gate.
func (p *CapabilityProfile) AllNAICSCodes() []string {
	seen := make(map[string]bool)
	var codes []string

	add := func(code string) {
		if code != "" && !seen[code] {
			seen[code] = true
			codes = append(codes, code)
		}
	}

	add(p.NAICS.Primary)
	for _, code := range p.NAICS.Secondary {
		add(code)
	}
	for _, code := range p.NAICS.Tertiary {
		add(code)
	}

	return codes
}

// LoadProfile reads a CapabilityProfile from a JSON or YAML file.
// The format is inferred from the file extension: .yaml and .yml are parsed as
// YAML; everything else (including .json) is parsed as JSON.
func LoadProfile(path string) (*CapabilityProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile %s: %w", path, err)
	}

	var p CapabilityProfile
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("failed to parse YAML profile %s: %w", path, err)
		}
	default:
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("failed to parse JSON profile %s: %w", path, err)
		}
	}

	return &p, nil
}
