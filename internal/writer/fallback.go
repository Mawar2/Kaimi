package writer

import (
	"context"
	"errors"
	"fmt"
)

// FallbackGenerator tries a chain of Generators in priority order and returns the
// first success. It lets a capable but preview model (gemini-3.1-pro-preview)
// draft while healthy, with a GA model (gemini-2.5-pro) catching preview outages
// — graceful degradation, never a stub.
type FallbackGenerator struct {
	generators []Generator
}

// NewFallbackGenerator chains generators in priority order (primary first). At
// least one generator should be supplied.
func NewFallbackGenerator(generators ...Generator) *FallbackGenerator {
	return &FallbackGenerator{generators: generators}
}

// GenerateSection tries each generator in order, returning the first success. It
// stops early if the context is done — a cancelled or expired context would fail
// every remaining generator too, so retrying is pointless. If all generators
// fail, it joins their errors so the cause of each is preserved.
func (f *FallbackGenerator) GenerateSection(ctx context.Context, systemInstruction, prompt string) (string, error) {
	if len(f.generators) == 0 {
		return "", fmt.Errorf("writer: fallback generator has no generators configured")
	}
	var errs []error
	for _, gen := range f.generators {
		if ctx.Err() != nil {
			return "", fmt.Errorf("writer: context done before the fallback chain completed: %w", ctx.Err())
		}
		text, err := gen.GenerateSection(ctx, systemInstruction, prompt)
		if err == nil {
			return text, nil
		}
		errs = append(errs, err)
	}
	return "", fmt.Errorf("writer: all %d generators in the fallback chain failed: %w", len(f.generators), errors.Join(errs...))
}
