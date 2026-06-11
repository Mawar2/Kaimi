package claudevertex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// vertexAnthropicVersion is the required version string for Anthropic models
	// on Vertex AI. It is distinct from the first-party Anthropic API version and
	// must be sent in the request body — the model id travels in the URL, never
	// in the body.
	vertexAnthropicVersion = "vertex-2023-10-16"

	// defaultMaxTokens is the output-token ceiling. It is large because proposal
	// sections and compliance reviews can run long; rawPredict is non-streaming,
	// so the HTTP client uses a generous timeout to match (see requestTimeout).
	defaultMaxTokens = 16384

	// cloudPlatformScope is the OAuth scope Application Default Credentials need
	// to call Vertex AI.
	cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

	// requestTimeout bounds a single non-streaming rawPredict call. Large outputs
	// at high effort can take a while, so this is deliberately generous.
	requestTimeout = 5 * time.Minute
)

// Generator calls a single Claude model on Vertex AI via rawPredict. It
// satisfies the Writer's Generator interface, so the Writer and Final Review
// can share one production model client.
type Generator struct {
	httpClient *http.Client
	endpoint   string // full rawPredict URL with the model id embedded
	model      string // retained for error context only
	maxTokens  int
	thinking   bool // when true, request adaptive thinking
}

// Option configures a Generator at construction time.
type Option func(*Generator)

// WithMaxTokens overrides the default output-token ceiling.
func WithMaxTokens(n int) Option {
	return func(g *Generator) {
		if n > 0 {
			g.maxTokens = n
		}
	}
}

// WithThinking enables adaptive thinking (the only on-mode for Opus 4.8 and
// Fable 5; a fixed budget_tokens is rejected with HTTP 400). Only text content
// blocks are ever returned — thinking blocks are skipped — so enabling this
// trades cost and latency for deeper reasoning without leaking reasoning into
// the drafted prose.
func WithThinking(enabled bool) Option {
	return func(g *Generator) { g.thinking = enabled }
}

// New constructs a Generator authenticated with Application Default Credentials.
// region is the Vertex location (e.g. "us-east5") and model is the publisher
// model id (e.g. "claude-opus-4-8", "claude-fable-5").
func New(ctx context.Context, projectID, region, model string, opts ...Option) (*Generator, error) {
	if projectID == "" || region == "" || model == "" {
		return nil, fmt.Errorf("claudevertex: projectID, region, and model are all required")
	}
	ts, err := google.DefaultTokenSource(ctx, cloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("claudevertex: load application default credentials: %w", err)
	}
	// oauth2.NewClient attaches a fresh bearer token to every request; set a
	// timeout on the returned client to bound the non-streaming call.
	httpClient := oauth2.NewClient(ctx, ts)
	httpClient.Timeout = requestTimeout

	endpoint := fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/anthropic/models/%s:rawPredict",
		region, projectID, region, model,
	)
	return newGenerator(httpClient, endpoint, model, opts...), nil
}

// newGenerator is the shared constructor used by New and by tests (which inject
// a local httptest client and endpoint instead of ADC).
func newGenerator(httpClient *http.Client, endpoint, model string, opts ...Option) *Generator {
	g := &Generator{
		httpClient: httpClient,
		endpoint:   endpoint,
		model:      model,
		maxTokens:  defaultMaxTokens,
	}
	for _, o := range opts {
		o(g)
	}
	return g
}

// requestBody is the Anthropic Messages payload as accepted by Vertex rawPredict.
// The model id is carried in the URL, so it is intentionally absent here.
type requestBody struct {
	AnthropicVersion string          `json:"anthropic_version"`
	MaxTokens        int             `json:"max_tokens"`
	System           string          `json:"system,omitempty"`
	Messages         []message       `json:"messages"`
	Thinking         *thinkingConfig `json:"thinking,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type thinkingConfig struct {
	Type string `json:"type"`
}

// responseBody captures the fields of the Anthropic Messages response we need.
type responseBody struct {
	Content    []contentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// GenerateSection implements the Writer's Generator interface: it sends the
// system instruction and user prompt to the configured Claude model on Vertex
// and returns the concatenated text. It never returns a silent empty string —
// a refusal, a non-200 status, or content with no text surfaces as an error.
func (g *Generator) GenerateSection(ctx context.Context, systemInstruction, prompt string) (string, error) {
	reqBody := requestBody{
		AnthropicVersion: vertexAnthropicVersion,
		MaxTokens:        g.maxTokens,
		System:           systemInstruction,
		Messages:         []message{{Role: "user", Content: prompt}},
	}
	if g.thinking {
		reqBody.Thinking = &thinkingConfig{Type: "adaptive"}
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("claudevertex: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("claudevertex: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("claudevertex: rawPredict call to %s: %w", g.model, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("claudevertex: read %s response: %w", g.model, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claudevertex: %s returned HTTP %d: %s", g.model, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed responseBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("claudevertex: decode %s response: %w", g.model, err)
	}

	// Safety classifiers may decline with a 200 and stop_reason "refusal"; the
	// content is empty or partial and must not be treated as a valid draft.
	if parsed.StopReason == "refusal" {
		return "", fmt.Errorf("claudevertex: %s declined the request (stop_reason=refusal)", g.model)
	}

	// Concatenate only text blocks; skip thinking blocks so reasoning never
	// leaks into the drafted prose.
	var sb strings.Builder
	for _, block := range parsed.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		return "", fmt.Errorf("claudevertex: %s returned no text content (stop_reason=%q)", g.model, parsed.StopReason)
	}
	return text, nil
}
