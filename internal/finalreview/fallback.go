package finalreview

import (
	"context"
	"errors"
	"fmt"
)

// FallbackChecker tries a chain of ComplianceCheckers in priority order and
// returns the first success. It lets a capable but preview model
// (gemini-3.1-pro-preview) verify compliance while healthy, with a GA model
// (gemini-2.5-pro) catching preview outages — graceful degradation on top of the
// deterministic checks that always run first in the agent.
type FallbackChecker struct {
	checkers []ComplianceChecker
}

// NewFallbackChecker chains checkers in priority order (primary first). At least
// one checker should be supplied.
func NewFallbackChecker(checkers ...ComplianceChecker) *FallbackChecker {
	return &FallbackChecker{checkers: checkers}
}

// CheckCompliance tries each checker in order, returning the first success. It
// stops early if the context is done, and joins all errors if every checker fails.
func (f *FallbackChecker) CheckCompliance(ctx context.Context, systemInstruction, prompt string) (string, error) {
	if len(f.checkers) == 0 {
		return "", fmt.Errorf("finalreview: fallback checker has no checkers configured")
	}
	var errs []error
	for _, c := range f.checkers {
		if ctx.Err() != nil {
			return "", fmt.Errorf("finalreview: context done before the fallback chain completed: %w", ctx.Err())
		}
		raw, err := c.CheckCompliance(ctx, systemInstruction, prompt)
		if err == nil {
			return raw, nil
		}
		errs = append(errs, err)
	}
	return "", fmt.Errorf("finalreview: all %d checkers in the fallback chain failed: %w", len(f.checkers), errors.Join(errs...))
}
