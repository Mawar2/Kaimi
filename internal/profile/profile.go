// Package profile defines BlueMeta Technologies' capability profile for evaluating
// federal contracting opportunities.
//
// The CapabilityProfile encodes company capabilities, certifications, and past
// performance. Hunter loads the profile from a JSON or YAML file via LoadProfile,
// then resolves NAICS codes via AllNAICSCodes for SAM.gov queries.
package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// NAICSTier represents the priority tier of a NAICS code.
// Primary codes are the strongest work-type match; tertiary are the weakest.
type NAICSTier string

const (
	// TierPrimary indicates the core NAICS codes BlueMeta performs most frequently.
	TierPrimary NAICSTier = "primary"

	// TierSecondary indicates NAICS codes BlueMeta can perform with strong capability.
	TierSecondary NAICSTier = "secondary"

	// TierTertiary indicates NAICS codes BlueMeta can perform but are not a core focus.
	TierTertiary NAICSTier = "tertiary"
)

// NAICSCode is a North American Industry Classification System code with its tier.
type NAICSCode struct {
	Code        string    `json:"code" yaml:"code"`
	Description string    `json:"description" yaml:"description"`
	Tier        NAICSTier `json:"tier" yaml:"tier"`
}

// SetAsideStatus represents eligibility for federal contracting set-aside programs.
// Fields reflect which certifications BlueMeta holds (true) or does not hold (false).
type SetAsideStatus struct {
	SmallBusiness bool `json:"small_business" yaml:"small_business"`
	SDB           bool `json:"sdb" yaml:"sdb"`
	EightA        bool `json:"eight_a" yaml:"eight_a"`
	SDVOSB        bool `json:"sdvosb" yaml:"sdvosb"`
	WOSB          bool `json:"wosb" yaml:"wosb"`
	HUBZone       bool `json:"hubzone" yaml:"hubzone"`
	VOSB          bool `json:"vosb" yaml:"vosb"`
}

// PastPerformance is a past project or contract entry (lightweight fact representation).
// Full narratives and embeddings are reserved for the Phase 3 knowledge base.
type PastPerformance struct {
	Client       string   `json:"client" yaml:"client"`
	Scope        string   `json:"scope" yaml:"scope"`
	Value        string   `json:"value" yaml:"value"`
	WhatItProves []string `json:"what_it_proves" yaml:"what_it_proves"`
}

// CapabilityProfile holds BlueMeta Technologies' certifications, NAICS codes,
// and past performance for federal contracting eligibility and fit scoring.
//
// Hunter loads this from profile.json (or a YAML equivalent) to resolve the NAICS
// code list for SAM.gov queries.
type CapabilityProfile struct {
	Company         string            `json:"company" yaml:"company"`
	NAICSCodes      []NAICSCode       `json:"naics_codes" yaml:"naics_codes"`
	SetAside        SetAsideStatus    `json:"set_aside" yaml:"set_aside"`
	PastPerformance []PastPerformance `json:"past_performance" yaml:"past_performance"`
}

// LoadProfile loads a CapabilityProfile from a JSON or YAML file.
// The format is determined by the file extension: .json, .yaml, or .yml.
func LoadProfile(path string) (*CapabilityProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file: %w", err)
	}

	var p CapabilityProfile
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".json":
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("failed to parse JSON profile: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("failed to parse YAML profile: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported profile format %q (use .json, .yaml, or .yml)", ext)
	}

	return &p, nil
}

// AllNAICSCodes returns a flat list of all NAICS code strings across all tiers,
// ordered as they appear in the profile (primary first by convention).
func (p *CapabilityProfile) AllNAICSCodes() []string {
	codes := make([]string, 0, len(p.NAICSCodes))
	for _, nc := range p.NAICSCodes {
		codes = append(codes, nc.Code)
	}
	return codes
}
