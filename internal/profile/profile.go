package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// NAICSTiers groups NAICS codes by match strength priority.
type NAICSTiers struct {
	Primary   []string `json:"primary" yaml:"primary"`
	Secondary []string `json:"secondary" yaml:"secondary"`
	Tertiary  []string `json:"tertiary" yaml:"tertiary"`
}

// SetAsideStatus defines the set-aside categories the company qualifies for.
type SetAsideStatus struct {
	SmallBusiness bool `json:"small_business" yaml:"small_business"`
	SDB           bool `json:"sdb" yaml:"sdb"` // Small Disadvantaged Business
	MinorityOwned bool `json:"minority_owned" yaml:"minority_owned"`
}

// PastPerformance represents a past contract project fact entry.
type PastPerformance struct {
	// ID is a unique reference identifier (e.g. "census").
	// Designed so a Phase 3 knowledge base (detailed narratives, embeddings/RAG)
	// can link or attach rich data to this entry using this ID.
	ID           string   `json:"id,omitempty" yaml:"id,omitempty"`
	Client       string   `json:"client" yaml:"client"`
	Scope        string   `json:"scope" yaml:"scope"`
	Value        float64  `json:"value" yaml:"value"`
	WhatItProves []string `json:"what_it_proves" yaml:"what_it_proves"`
}

// CapabilityProfile holds the structured capabilities and facts about the firm.
type CapabilityProfile struct {
	UEI             string            `json:"uei" yaml:"uei"`
	CAGE            string            `json:"cage" yaml:"cage"`
	NAICS           NAICSTiers        `json:"naics" yaml:"naics"`
	SetAside        SetAsideStatus    `json:"set_aside" yaml:"set_aside"`
	ClearanceStatus string            `json:"clearance_status" yaml:"clearance_status"`
	CompetencyTags  []string          `json:"competency_tags" yaml:"competency_tags"`
	PastPerformance []PastPerformance `json:"past_performance" yaml:"past_performance"`
}

// LoadProfile loads a CapabilityProfile from a JSON or YAML file path.
func LoadProfile(path string) (*CapabilityProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file: %w", err)
	}

	var prof CapabilityProfile
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".yaml" || ext == ".yml" {
		if err := yaml.Unmarshal(data, &prof); err != nil {
			return nil, fmt.Errorf("failed to unmarshal YAML profile: %w", err)
		}
	} else {
		// Default to JSON
		if err := json.Unmarshal(data, &prof); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON profile: %w", err)
		}
	}

	return &prof, nil
}
