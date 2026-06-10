package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/scorer"
)

// scorerCaseFile is the on-disk JSON shape of one scorer eval case. The Opportunity
// is referenced by fixture filename (resolved against a base directory) so cases
// reuse the existing opportunity fixtures rather than duplicating their content.
type scorerCaseFile struct {
	Name                   string                `json:"name"`
	OpportunityFixture     string                `json:"opportunity_fixture"`
	ExpectedRecommendation scorer.Recommendation `json:"expected_recommendation"`
	ExpectedReasonKeywords []string              `json:"expected_reason_keywords,omitempty"`
}

// writerCaseFile is the on-disk JSON shape of one writer groundedness eval case.
type writerCaseFile struct {
	Name              string   `json:"name"`
	SectionPrompt     string   `json:"section_prompt"`
	SystemInstruction string   `json:"system_instruction,omitempty"`
	Facts             []string `json:"facts"`
	MustNotFabricate  []string `json:"must_not_fabricate,omitempty"`
}

// LoadScorerCases reads a JSON array of scorer cases from path and resolves each
// case's opportunity_fixture (a filename) into an Opportunity loaded from fixtureDir.
//
// Keeping the fixture reference relative to fixtureDir lets the same dataset run from
// any working directory by passing the correct base directory.
func LoadScorerCases(path, fixtureDir string) ([]ScorerCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("eval: read scorer dataset %s: %w", path, err)
	}

	var raw []scorerCaseFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("eval: parse scorer dataset %s: %w", path, err)
	}

	cases := make([]ScorerCase, 0, len(raw))
	for i, rc := range raw {
		if rc.Name == "" {
			return nil, fmt.Errorf("eval: scorer case %d has empty name", i)
		}
		if rc.OpportunityFixture == "" {
			return nil, fmt.Errorf("eval: scorer case %q has empty opportunity_fixture", rc.Name)
		}

		opp, err := loadOpportunityFixture(filepath.Join(fixtureDir, rc.OpportunityFixture))
		if err != nil {
			return nil, fmt.Errorf("eval: scorer case %q: %w", rc.Name, err)
		}

		cases = append(cases, ScorerCase{
			Name:                   rc.Name,
			Opportunity:            opp,
			ExpectedRecommendation: rc.ExpectedRecommendation,
			ExpectedReasonKeywords: rc.ExpectedReasonKeywords,
		})
	}
	return cases, nil
}

// LoadWriterCases reads a JSON array of writer groundedness cases from path.
func LoadWriterCases(path string) ([]WriterCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("eval: read writer dataset %s: %w", path, err)
	}

	var raw []writerCaseFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("eval: parse writer dataset %s: %w", path, err)
	}

	cases := make([]WriterCase, 0, len(raw))
	for i, rc := range raw {
		if rc.Name == "" {
			return nil, fmt.Errorf("eval: writer case %d has empty name", i)
		}
		// writerCaseFile and WriterCase have identical fields (only struct tags
		// differ), so a direct conversion is valid and clearer than a field copy.
		cases = append(cases, WriterCase(rc))
	}
	return cases, nil
}

// loadOpportunityFixture reads a single Opportunity from a JSON file.
func loadOpportunityFixture(path string) (*opportunity.Opportunity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read opportunity fixture %s: %w", path, err)
	}
	var opp opportunity.Opportunity
	if err := json.Unmarshal(data, &opp); err != nil {
		return nil, fmt.Errorf("parse opportunity fixture %s: %w", path, err)
	}
	return &opp, nil
}
