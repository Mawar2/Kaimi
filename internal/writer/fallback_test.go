package writer

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// stubGen is a Generator test double that returns a fixed text/error and records
// whether it was called.
type stubGen struct {
	text   string
	err    error
	called bool
}

func (s *stubGen) GenerateSection(_ context.Context, _, _ string) (string, error) {
	s.called = true
	return s.text, s.err
}

func TestFallbackGenerator_PrimarySuccess_SkipsFallback(t *testing.T) {
	primary := &stubGen{text: "primary draft"}
	fallback := &stubGen{text: "fallback draft"}

	got, err := NewFallbackGenerator(primary, fallback).GenerateSection(context.Background(), "sys", "p")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != "primary draft" {
		t.Errorf("text = %q, want primary draft", got)
	}
	if fallback.called {
		t.Error("fallback should not be called when the primary succeeds")
	}
}

func TestFallbackGenerator_PrimaryFails_UsesFallback(t *testing.T) {
	primary := &stubGen{err: errors.New("preview model unavailable")}
	fallback := &stubGen{text: "GA draft"}

	got, err := NewFallbackGenerator(primary, fallback).GenerateSection(context.Background(), "sys", "p")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != "GA draft" {
		t.Errorf("text = %q, want the GA fallback draft", got)
	}
	if !fallback.called {
		t.Error("fallback should be called when the primary fails")
	}
}

func TestFallbackGenerator_AllFail_JoinsErrors(t *testing.T) {
	primary := &stubGen{err: errors.New("preview down")}
	fallback := &stubGen{err: errors.New("GA down")}

	_, err := NewFallbackGenerator(primary, fallback).GenerateSection(context.Background(), "sys", "p")
	if err == nil {
		t.Fatal("expected an error when every generator fails")
	}
	if !strings.Contains(err.Error(), "preview down") || !strings.Contains(err.Error(), "GA down") {
		t.Errorf("joined error should mention both causes, got: %v", err)
	}
}

func TestFallbackGenerator_ContextDone_StopsEarly(t *testing.T) {
	primary := &stubGen{err: errors.New("primary error")}
	fallback := &stubGen{text: "should not run"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // context is already done before the first attempt

	_, err := NewFallbackGenerator(primary, fallback).GenerateSection(ctx, "sys", "p")
	if err == nil {
		t.Fatal("expected an error on a cancelled context")
	}
	if fallback.called {
		t.Error("fallback must not run once the context is done")
	}
}
