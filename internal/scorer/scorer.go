package scorer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/store"
)

const (
	// scorerModel is the Gemini model used for scoring.
	// TODO(phase-1): upgrade to gemini-3-pro once GA on Vertex AI us-east4.
	scorerModel = "gemini-2.5-pro"

	// projectID and location match the project constants from the spike.
	projectID = "kaimi-seeker"
	location  = "us-east4"
)

// Recommendation is the Scorer's bid/no-bid decision.
type Recommendation string

const (
	// RecommendationBid means the opportunity is a strong fit — bid it.
	RecommendationBid Recommendation = "BID"
	// RecommendationNoBid means the opportunity is a poor fit — do not bid.
	RecommendationNoBid Recommendation = "NO_BID"
	// RecommendationReview means the opportunity is borderline — needs human review.
	RecommendationReview Recommendation = "REVIEW"
)

// ScoreResult is the complete output of a scoring pass.
type ScoreResult struct {
	// Score is the bid/no-bid fit score from 0 (no fit) to 100 (perfect fit).
	Score int
	// Recommendation is the derived bid/no-bid recommendation.
	Recommendation Recommendation
	// Reasoning is a human-readable explanation of the score, suitable for
	// display in the Opportunity Queue dashboard.
	Reasoning string
	// Requirements are the must-have requirements extracted from the solicitation.
	Requirements []string
}

// Scorer evaluates an opportunity against a capability profile.
type Scorer interface {
	// Score returns a ScoreResult for the given opportunity and profile.
	// Returns an error if scoring fails (e.g., LLM call failure, invalid response).
	Score(ctx context.Context, opp *opportunity.Opportunity, profile *CapabilityProfile) (*ScoreResult, error)
}

// ScoreAndSave scores the opportunity and persists the enriched record to the store.
//
// It updates Opportunity.Score (0.0-1.0), Recommendation, ScoreReasoning,
// Requirements, ScoredAt, and UpdatedAt before saving.
func ScoreAndSave(ctx context.Context, scorer Scorer, s store.Store, opp *opportunity.Opportunity, profile *CapabilityProfile) (*ScoreResult, error) {
	if opp == nil {
		return nil, fmt.Errorf("opportunity cannot be nil")
	}
	if profile == nil {
		return nil, fmt.Errorf("capability profile cannot be nil")
	}

	result, err := scorer.Score(ctx, opp, profile)
	if err != nil {
		return nil, fmt.Errorf("scoring opportunity %s: %w", opp.ID, err)
	}

	now := time.Now().UTC()
	opp.Score = float64(result.Score) / 100.0
	opp.Recommendation = string(result.Recommendation)
	opp.ScoreReasoning = result.Reasoning
	opp.Requirements = result.Requirements
	opp.ScoredAt = &now
	opp.UpdatedAt = now

	if err := s.Save(ctx, opp); err != nil {
		return nil, fmt.Errorf("saving scored opportunity %s: %w", opp.ID, err)
	}

	return result, nil
}

// GeminiScorer implements Scorer using Gemini 2.5 Pro via Vertex AI.
type GeminiScorer struct {
	llm llmClient
}

// NewGeminiScorer creates a GeminiScorer using Application Default Credentials.
//
// Call `gcloud auth application-default login` once before running in live mode.
func NewGeminiScorer(ctx context.Context) (*GeminiScorer, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendEnterprise,
		Project:  projectID,
		Location: location,
	})
	if err != nil {
		return nil, fmt.Errorf("creating Gemini client: %w", err)
	}

	return &GeminiScorer{
		llm: &geminiLLMClient{client: client},
	}, nil
}

// Score implements Scorer using Gemini via Vertex AI.
func (s *GeminiScorer) Score(ctx context.Context, opp *opportunity.Opportunity, profile *CapabilityProfile) (*ScoreResult, error) {
	if opp == nil {
		return nil, fmt.Errorf("opportunity cannot be nil")
	}
	if profile == nil {
		return nil, fmt.Errorf("capability profile cannot be nil")
	}

	sig := computeSignals(opp, profile)
	prompt := buildScoringPrompt(opp, profile, sig)
	return s.llm.generateContent(ctx, prompt)
}

// signals holds pre-computed deterministic matching signals.
// These are computed before the LLM call so the model can focus on synthesis,
// not raw string matching.
type signals struct {
	primaryNAICSMatch   bool
	secondaryNAICSMatch bool
	matchingTags        []string
	pastPerfKeywords    []string
	sdbApplicable       bool
}

// computeSignals pre-computes deterministic matching signals from the opportunity
// and profile. These are passed to the LLM as structured context.
func computeSignals(opp *opportunity.Opportunity, profile *CapabilityProfile) signals {
	var sig signals

	// NAICS match: primary beats secondary
	for _, code := range profile.PrimaryNAICS {
		if code == opp.NAICSCode {
			sig.primaryNAICSMatch = true
			break
		}
	}
	if !sig.primaryNAICSMatch {
		for _, code := range profile.SecondaryNAICS {
			if code == opp.NAICSCode {
				sig.secondaryNAICSMatch = true
				break
			}
		}
	}

	// Competency tag overlap: case-insensitive substring match against title+description
	descLower := strings.ToLower(opp.Title + " " + opp.Description)
	for _, tag := range profile.CompetencyTags {
		if strings.Contains(descLower, strings.ToLower(tag)) {
			sig.matchingTags = append(sig.matchingTags, tag)
		}
	}

	// Past performance keyword overlap
	for _, kw := range profile.PastPerformance {
		if strings.Contains(descLower, strings.ToLower(kw)) {
			sig.pastPerfKeywords = append(sig.pastPerfKeywords, kw)
		}
	}

	// SDB is a positive factor only when the solicitation has a matching set-aside
	if profile.IsSDB {
		for _, code := range profile.SetAsideCodes {
			if code == opp.SetAsideCode {
				sig.sdbApplicable = true
				break
			}
		}
	}

	return sig
}

// buildScoringPrompt assembles the Gemini prompt combining the opportunity text,
// capability profile, and pre-computed signals.
func buildScoringPrompt(opp *opportunity.Opportunity, profile *CapabilityProfile, sig signals) string {
	var sb strings.Builder

	sb.WriteString("You are a federal contracting business development analyst. Score the following opportunity for bid/no-bid fit.\n\n")

	sb.WriteString("## Company Capability Profile\n")
	fmt.Fprintf(&sb, "Company: %s\n", profile.CompanyName)
	fmt.Fprintf(&sb, "Primary NAICS codes (highest relevance): %s\n", strings.Join(profile.PrimaryNAICS, ", "))
	if len(profile.SecondaryNAICS) > 0 {
		fmt.Fprintf(&sb, "Secondary NAICS codes (moderate relevance): %s\n", strings.Join(profile.SecondaryNAICS, ", "))
	}
	fmt.Fprintf(&sb, "Core competencies: %s\n", strings.Join(profile.CompetencyTags, ", "))
	fmt.Fprintf(&sb, "Past performance areas: %s\n", strings.Join(profile.PastPerformance, ", "))
	if profile.IsSDB {
		sb.WriteString("SDB Status: Yes — Small Disadvantaged Business\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Opportunity\n")
	fmt.Fprintf(&sb, "Title: %s\n", opp.Title)
	fmt.Fprintf(&sb, "Agency: %s", opp.Agency)
	if opp.Office != "" {
		fmt.Fprintf(&sb, " / %s", opp.Office)
	}
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "NAICS: %s", opp.NAICSCode)
	if opp.NAICSDescription != "" {
		fmt.Fprintf(&sb, " (%s)", opp.NAICSDescription)
	}
	sb.WriteString("\n")
	if opp.SetAsideCode != "" {
		fmt.Fprintf(&sb, "Set-Aside: %s\n", opp.SetAsideCode)
	}
	fmt.Fprintf(&sb, "Type: %s\n", opp.Type)
	fmt.Fprintf(&sb, "Description:\n%s\n\n", opp.Description)

	sb.WriteString("## Pre-Computed Signals\n")
	switch {
	case sig.primaryNAICSMatch:
		sb.WriteString("✓ PRIMARY NAICS match (highest weight signal)\n")
	case sig.secondaryNAICSMatch:
		sb.WriteString("~ Secondary NAICS match (moderate weight signal)\n")
	default:
		sb.WriteString("✗ No NAICS match (strong negative signal)\n")
	}
	if len(sig.matchingTags) > 0 {
		fmt.Fprintf(&sb, "✓ Competency overlaps: %s\n", strings.Join(sig.matchingTags, ", "))
	} else {
		sb.WriteString("✗ No competency tag overlap\n")
	}
	if len(sig.pastPerfKeywords) > 0 {
		fmt.Fprintf(&sb, "✓ Past performance relevance: %s\n", strings.Join(sig.pastPerfKeywords, ", "))
	}
	if sig.sdbApplicable {
		sb.WriteString("✓ SDB set-aside advantage applies\n")
	}
	sb.WriteString("\n")

	sb.WriteString(`## Scoring Instructions

Assign a score 0–100 using these weights:
- Primary NAICS match: +30–40 points
- Secondary NAICS match only: +15–20 points
- No NAICS match: cap total at 30 points
- Each competency tag overlap: +5 points (cap at +25)
- Past performance relevance: +5–10 points total
- SDB set-aside advantage: +5–10 points
- Missing must-have requirements: −10–20 points each

Derive the recommendation from the score:
- BID    → score ≥ 70
- REVIEW → score 50–69
- NO_BID → score < 50

Extract the 2–5 most important must-have requirements from the description.

Return ONLY valid JSON — no markdown, no preamble:
{
  "score": <integer 0–100>,
  "recommendation": "<BID|NO_BID|REVIEW>",
  "reasoning": "<2–4 sentences explaining the score — cite specific signals>",
  "requirements": ["<requirement 1>", "<requirement 2>"]
}`)

	return sb.String()
}

// scoreResponse is the raw JSON structure returned by the LLM.
type scoreResponse struct {
	Score          int      `json:"score"`
	Recommendation string   `json:"recommendation"`
	Reasoning      string   `json:"reasoning"`
	Requirements   []string `json:"requirements"`
}

// llmClient abstracts the LLM call, enabling deterministic mocks in unit tests.
type llmClient interface {
	generateContent(ctx context.Context, prompt string) (*ScoreResult, error)
}

// geminiLLMClient wraps genai.Client for the scorer.
type geminiLLMClient struct {
	client *genai.Client
}

// generateContent calls Gemini and parses the structured JSON response.
func (g *geminiLLMClient) generateContent(ctx context.Context, prompt string) (*ScoreResult, error) {
	config := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"score": {
					Type: genai.TypeInteger,
				},
				"recommendation": {
					Type: genai.TypeString,
					Enum: []string{"BID", "NO_BID", "REVIEW"},
				},
				"reasoning": {
					Type: genai.TypeString,
				},
				"requirements": {
					Type:  genai.TypeArray,
					Items: &genai.Schema{Type: genai.TypeString},
				},
			},
			Required: []string{"score", "recommendation", "reasoning", "requirements"},
		},
	}

	resp, err := g.client.Models.GenerateContent(ctx, scorerModel, genai.Text(prompt), config)
	if err != nil {
		return nil, fmt.Errorf("calling Gemini: %w", err)
	}

	text := resp.Text()
	if text == "" {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	var raw scoreResponse
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, fmt.Errorf("parsing Gemini response: %w\nresponse was: %s", err, text)
	}

	return validateAndConvert(&raw)
}

// validateAndConvert validates the raw LLM response fields and returns a ScoreResult.
func validateAndConvert(raw *scoreResponse) (*ScoreResult, error) {
	if raw.Score < 0 || raw.Score > 100 {
		return nil, fmt.Errorf("score %d is outside valid range 0–100", raw.Score)
	}

	rec := Recommendation(raw.Recommendation)
	switch rec {
	case RecommendationBid, RecommendationNoBid, RecommendationReview:
		// valid
	default:
		return nil, fmt.Errorf("invalid recommendation %q: must be BID, NO_BID, or REVIEW", raw.Recommendation)
	}

	if strings.TrimSpace(raw.Reasoning) == "" {
		return nil, fmt.Errorf("reasoning cannot be empty")
	}

	reqs := raw.Requirements
	if reqs == nil {
		reqs = []string{}
	}

	return &ScoreResult{
		Score:          raw.Score,
		Recommendation: rec,
		Reasoning:      raw.Reasoning,
		Requirements:   reqs,
	}, nil
}
