package claudevertex

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Mawar2/Kaimi/internal/writer"
)

// The production ClaudeVertexGenerator must satisfy the Writer's Generator
// interface so the Writer and Final Review can share one model client. This is
// a compile-time assertion only (kept in the test so production code does not
// import the writer package).
var _ writer.Generator = (*Generator)(nil)

// newTestGenerator wires a Generator against a local httptest server, bypassing
// Application Default Credentials (which only the live-tagged test exercises).
func newTestGenerator(t *testing.T, handler http.HandlerFunc, opts ...Option) (*Generator, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return newGenerator(srv.Client(), srv.URL, "claude-opus-4-8", opts...), srv
}

// textResponse is a minimal valid Anthropic Messages response body.
func textResponse(t *testing.T, text string) []byte {
	t.Helper()
	body := map[string]any{
		"id":          "msg_test",
		"type":        "message",
		"role":        "assistant",
		"stop_reason": "end_turn",
		"content":     []map[string]any{{"type": "text", "text": text}},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return raw
}

func TestGenerateSection_SendsVertexEnvelope(t *testing.T) {
	var captured map[string]any
	gen, _ := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Errorf("request body not JSON: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(textResponse(t, "Drafted section prose."))
	})

	got, err := gen.GenerateSection(context.Background(), "system rules", "draft the technical approach")
	if err != nil {
		t.Fatalf("GenerateSection returned error: %v", err)
	}
	if got != "Drafted section prose." {
		t.Errorf("text = %q, want %q", got, "Drafted section prose.")
	}

	// The Vertex envelope requires anthropic_version in the body and the model
	// id only in the URL (never in the body).
	if captured["anthropic_version"] != "vertex-2023-10-16" {
		t.Errorf("anthropic_version = %v, want vertex-2023-10-16", captured["anthropic_version"])
	}
	if _, present := captured["model"]; present {
		t.Errorf("model must not appear in the Vertex request body, got %v", captured["model"])
	}
	if captured["system"] != "system rules" {
		t.Errorf("system = %v, want %q", captured["system"], "system rules")
	}
	if captured["max_tokens"].(float64) != float64(defaultMaxTokens) {
		t.Errorf("max_tokens = %v, want %d", captured["max_tokens"], defaultMaxTokens)
	}
	msgs, ok := captured["messages"].([]any)
	if !ok || len(msgs) != 1 {
		t.Fatalf("messages = %v, want one entry", captured["messages"])
	}
	first := msgs[0].(map[string]any)
	if first["role"] != "user" || first["content"] != "draft the technical approach" {
		t.Errorf("messages[0] = %v, want user/draft prompt", first)
	}
	// Thinking is off by default.
	if _, present := captured["thinking"]; present {
		t.Errorf("thinking must be absent by default, got %v", captured["thinking"])
	}
}

func TestGenerateSection_SkipsThinkingBlocks(t *testing.T) {
	gen, _ := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{
			"type":        "message",
			"role":        "assistant",
			"stop_reason": "end_turn",
			"content": []map[string]any{
				{"type": "thinking", "thinking": "internal reasoning that must not leak"},
				{"type": "text", "text": "Final prose only."},
			},
		}
		raw, _ := json.Marshal(body)
		_, _ = w.Write(raw)
	})

	got, err := gen.GenerateSection(context.Background(), "sys", "prompt")
	if err != nil {
		t.Fatalf("GenerateSection returned error: %v", err)
	}
	if got != "Final prose only." {
		t.Errorf("text = %q, want only the text block content", got)
	}
}

func TestGenerateSection_WithThinkingEnablesAdaptive(t *testing.T) {
	var captured map[string]any
	gen, _ := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		_, _ = w.Write(textResponse(t, "ok"))
	}, WithThinking(true))

	if _, err := gen.GenerateSection(context.Background(), "sys", "prompt"); err != nil {
		t.Fatalf("GenerateSection returned error: %v", err)
	}
	thinking, ok := captured["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("thinking config missing, got %v", captured["thinking"])
	}
	if thinking["type"] != "adaptive" {
		t.Errorf("thinking.type = %v, want adaptive (budget_tokens 400s on Opus 4.8/Fable 5)", thinking["type"])
	}
}

func TestGenerateSection_WithMaxTokensOverride(t *testing.T) {
	var captured map[string]any
	gen, _ := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		_, _ = w.Write(textResponse(t, "ok"))
	}, WithMaxTokens(4096))

	if _, err := gen.GenerateSection(context.Background(), "sys", "prompt"); err != nil {
		t.Fatalf("GenerateSection returned error: %v", err)
	}
	if captured["max_tokens"].(float64) != 4096 {
		t.Errorf("max_tokens = %v, want 4096", captured["max_tokens"])
	}
}

func TestGenerateSection_ErrorsOnNon200(t *testing.T) {
	gen, _ := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"quota exceeded"}}`))
	})

	_, err := gen.GenerateSection(context.Background(), "sys", "prompt")
	if err == nil {
		t.Fatal("expected an error on HTTP 429, got nil")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error %q should mention the status code", err)
	}
}

func TestGenerateSection_ErrorsOnRefusal(t *testing.T) {
	gen, _ := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{
			"type":        "message",
			"role":        "assistant",
			"stop_reason": "refusal",
			"content":     []map[string]any{},
		}
		raw, _ := json.Marshal(body)
		_, _ = w.Write(raw)
	})

	_, err := gen.GenerateSection(context.Background(), "sys", "prompt")
	if err == nil {
		t.Fatal("expected an error on stop_reason=refusal, got nil")
	}
	if !strings.Contains(err.Error(), "refusal") {
		t.Errorf("error %q should mention the refusal", err)
	}
}

func TestGenerateSection_ErrorsOnEmptyText(t *testing.T) {
	gen, _ := newTestGenerator(t, func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{
			"type":        "message",
			"role":        "assistant",
			"stop_reason": "end_turn",
			"content":     []map[string]any{{"type": "text", "text": "   "}},
		}
		raw, _ := json.Marshal(body)
		_, _ = w.Write(raw)
	})

	_, err := gen.GenerateSection(context.Background(), "sys", "prompt")
	if err == nil {
		t.Fatal("expected an error on whitespace-only content, got nil")
	}
}
