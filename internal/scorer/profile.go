package scorer

// CapabilityProfile defines the company's capabilities for bid/no-bid scoring.
//
// The profile is the authoritative source of what the company can bid on.
// It is loaded once at startup and passed to every Score call.
type CapabilityProfile struct {
	// CompanyName is used in prompts and reports.
	CompanyName string `json:"company_name"`

	// PrimaryNAICS are the codes the company is most strongly aligned to.
	// A primary match carries the highest scoring weight.
	PrimaryNAICS []string `json:"primary_naics"`

	// SecondaryNAICS are related codes the company can perform but is not core.
	// A secondary match carries moderate scoring weight.
	SecondaryNAICS []string `json:"secondary_naics"`

	// CompetencyTags are technical capability keywords matched against
	// opportunity description and title (case-insensitive substring match).
	CompetencyTags []string `json:"competency_tags"`

	// PastPerformance is a list of agency names or capability areas that
	// represent the company's past performance record.
	PastPerformance []string `json:"past_performance"`

	// IsSDB indicates whether the company holds SDB (Small Disadvantaged Business)
	// certification. When true and the set-aside matches, the scorer applies a
	// positive SDB factor.
	IsSDB bool `json:"is_sdb"`

	// SetAsideCodes lists the set-aside codes for which the company qualifies
	// (e.g., "SBA", "8A"). Used to determine whether the SDB factor applies.
	SetAsideCodes []string `json:"set_aside_codes"`
}
