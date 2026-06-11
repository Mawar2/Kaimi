package outline

import (
	"context"
	"errors"
	"fmt"

	"github.com/Mawar2/Kaimi/internal/opportunity"
)

// FallbackPlanner tries a chain of SectionPlanners in priority order and returns
// the first that yields sections. It always ends with the deterministic planner,
// so section planning never fails outright — if the Gemini planner is down, the
// rule-based structure still produces a usable outline.
type FallbackPlanner struct {
	planners []SectionPlanner
}

// NewFallbackPlanner chains the given planners in priority order and appends the
// deterministic planner as the final, always-succeeding fallback.
func NewFallbackPlanner(planners ...SectionPlanner) *FallbackPlanner {
	return &FallbackPlanner{planners: append(planners, deterministicPlanner{})}
}

// PlanSections tries each planner in order, returning the first non-empty result.
// It stops early if the context is done. Because the deterministic planner is
// always last and never errors, this returns sections unless the context is dead.
func (f *FallbackPlanner) PlanSections(ctx context.Context, opp *opportunity.Opportunity, source string) ([]Section, error) {
	var errs []error
	for _, p := range f.planners {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("outline: context done before the fallback chain completed: %w", ctx.Err())
		}
		sections, err := p.PlanSections(ctx, opp, source)
		if err == nil && len(sections) > 0 {
			return sections, nil
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	return nil, fmt.Errorf("outline: every planner in the fallback chain failed to produce sections: %w", errors.Join(errs...))
}
