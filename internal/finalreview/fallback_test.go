package finalreview

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type stubChecker struct {
	raw    string
	err    error
	called bool
}

func (s *stubChecker) CheckCompliance(_ context.Context, _, _ string) (string, error) {
	s.called = true
	return s.raw, s.err
}

func TestFallbackChecker_PrimarySuccess_SkipsFallback(t *testing.T) {
	primary := &stubChecker{raw: `{"pass":true}`}
	fallback := &stubChecker{raw: `{"pass":false}`}

	got, err := NewFallbackChecker(primary, fallback).CheckCompliance(context.Background(), "sys", "p")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != `{"pass":true}` {
		t.Errorf("raw = %q, want the primary verdict", got)
	}
	if fallback.called {
		t.Error("fallback should not run when the primary succeeds")
	}
}

func TestFallbackChecker_PrimaryFails_UsesFallback(t *testing.T) {
	primary := &stubChecker{err: errors.New("preview model down")}
	fallback := &stubChecker{raw: `{"pass":true}`}

	got, err := NewFallbackChecker(primary, fallback).CheckCompliance(context.Background(), "sys", "p")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != `{"pass":true}` || !fallback.called {
		t.Errorf("expected the GA fallback verdict; got %q called=%v", got, fallback.called)
	}
}

func TestFallbackChecker_AllFail_JoinsErrors(t *testing.T) {
	primary := &stubChecker{err: errors.New("preview down")}
	fallback := &stubChecker{err: errors.New("GA down")}

	_, err := NewFallbackChecker(primary, fallback).CheckCompliance(context.Background(), "sys", "p")
	if err == nil {
		t.Fatal("expected an error when every checker fails")
	}
	if !strings.Contains(err.Error(), "preview down") || !strings.Contains(err.Error(), "GA down") {
		t.Errorf("joined error should mention both causes, got: %v", err)
	}
}
