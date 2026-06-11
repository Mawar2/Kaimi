//go:build live

// Live verification that the GA fallback actually degrades gracefully against real
// Vertex endpoints: a deliberately-bogus primary model 404s, and the GA fallback
// (gemini-2.5-pro, regional) serves real prose instead. Run:
//
//	GCP_PROJECT_ID=kaimi-seeker \
//	  go test -tags live -run TestLiveFallback ./internal/writer
//
// Requires Application Default Credentials.
package writer

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveFallbackDegradesToGA(t *testing.T) {
	project := os.Getenv("GCP_PROJECT_ID")
	if project == "" {
		t.Skip("set GCP_PROJECT_ID to run the live fallback test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Primary model id does not exist → its call 404s, forcing the fallback.
	primary, err := NewGeminiGenerator(ctx, project, "global", "gemini-does-not-exist-preview")
	if err != nil {
		t.Fatalf("primary generator: %v", err)
	}
	// GA fallback: gemini-2.5-pro is GA and served regionally.
	fallback, err := NewGeminiGenerator(ctx, project, "us-east4", "gemini-2.5-pro")
	if err != nil {
		t.Fatalf("fallback generator: %v", err)
	}

	got, err := NewFallbackGenerator(primary, fallback).GenerateSection(ctx,
		"You write one short sentence and nothing else.",
		"Write a single sentence confirming the proposal pipeline is online.",
	)
	if err != nil {
		t.Fatalf("fallback chain failed to degrade to the GA model: %v", err)
	}
	if strings.TrimSpace(got) == "" {
		t.Fatal("GA fallback returned empty text")
	}
	t.Logf("primary 404'd; GA fallback (gemini-2.5-pro) served: %q", got)
}
