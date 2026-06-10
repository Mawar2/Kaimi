package writer

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

// GeminiGenerator is the Vertex AI / Gemini implementation of Generator.
//
// It mirrors scorer.GeminiScorer: a Vertex AI (BackendEnterprise) client with
// Application Default Credentials, called at low temperature for consistent,
// grounded prose. The anti-fabrication grounding lives in the prompt the Writer
// builds (buildSectionPrompt); this type only performs the model call.
type GeminiGenerator struct {
	client    *genai.Client
	modelName string
}

// NewGeminiGenerator creates a GeminiGenerator backed by Vertex AI.
//
// Requires Application Default Credentials (gcloud auth application-default login).
func NewGeminiGenerator(ctx context.Context, projectID, location, modelName string) (*GeminiGenerator, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendEnterprise,
		Project:  projectID,
		Location: location,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}
	return &GeminiGenerator{client: client, modelName: modelName}, nil
}

// Output-budget sizing for a section draft. gemini-2.5-pro is a thinking model:
// its reasoning tokens are billed against MaxOutputTokens, so the old 2048 cap
// was consumed by thinking and the longest sections truncated mid-sentence (see
// issue #192). We raise the cap and bound the thinking budget so reasoning can
// never starve the prose. Values are generous, tunable headroom.
const (
	maxSectionOutputTokens = 8192
	sectionThinkingBudget  = 2048
)

// sectionGenerateConfig builds the Gemini config for one section draft, with
// thinking-token headroom (see #192) and the anti-fabrication system instruction.
func sectionGenerateConfig(systemInstruction string) *genai.GenerateContentConfig {
	temp := float32(0.3) // low temperature: grounded, consistent prose
	budget := int32(sectionThinkingBudget)
	return &genai.GenerateContentConfig{
		Temperature:       &temp,
		MaxOutputTokens:   maxSectionOutputTokens,
		SystemInstruction: genai.NewContentFromText(systemInstruction, genai.RoleUser),
		ThinkingConfig:    &genai.ThinkingConfig{ThinkingBudget: &budget},
	}
}

// GenerateSection implements Generator using Gemini via Vertex AI.
//
// The anti-fabrication rules are passed as a system instruction (not in the user
// prompt) so they resist instruction drift on long opportunity text.
func (g *GeminiGenerator) GenerateSection(ctx context.Context, systemInstruction, prompt string) (string, error) {
	contents := []*genai.Content{
		genai.NewContentFromText(prompt, genai.RoleUser),
	}

	config := sectionGenerateConfig(systemInstruction)

	resp, err := g.client.Models.GenerateContent(ctx, g.modelName, contents, config)
	if err != nil {
		return "", fmt.Errorf("gemini API call failed: %w", err)
	}

	// A safety-blocked or otherwise empty response can have zero candidates;
	// guard before reading text so a blocked generation surfaces as an error.
	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("gemini returned no candidates (possibly safety-blocked)")
	}

	text := resp.Text()
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("gemini returned empty response")
	}
	return text, nil
}
