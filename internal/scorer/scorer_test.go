package scorer

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/store"
)

// testProfile returns a realistic CapabilityProfile fixture for BlueMeta Technologies.
func testProfile() *CapabilityProfile {
	return &CapabilityProfile{
		CompanyName:    "BlueMeta Technologies",
		PrimaryNAICS:   []string{"541512", "541519"},
		SecondaryNAICS: []string{"541511", "541513", "518210"},
		CompetencyTags: []string{
			"cloud", "AWS", "GCP", "Azure", "Kubernetes", "DevOps",
			"cybersecurity", "AI", "ML", "data analytics", "modernization",
			"infrastructure", "migration",
		},
		PastPerformance: []string{
			"GSA", "DHS", "DOE", "cloud migration", "infrastructure modernization",
		},
		IsSDB:         true,
		SetAsideCodes: []string{"SBA", "8A"},
	}
}

// testOpportunity returns a fixture Opportunity for testing.
func testOpportunity() *opportunity.Opportunity {
	now := time.Now().UTC()
	return &opportunity.Opportunity{
		ID:               "test-opp-001",
		Title:            "Cloud Infrastructure Modernization Services",
		Agency:           "GENERAL SERVICES ADMINISTRATION",
		Office:           "GSA/FAS/AAS",
		NAICSCode:        "541512",
		NAICSDescription: "Computer Systems Design Services",
		SetAsideCode:     "SBA",
		Description:      "Cloud infrastructure modernization services including migration to GCP, Kubernetes containerization, and DevOps automation pipeline implementation.",
		Type:             "Solicitation",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// mockScorer is a deterministic Scorer for unit tests.
type mockScorer struct {
	result *ScoreResult
	err    error
}

// Score implements Scorer.
func (m *mockScorer) Score(_ context.Context, _ *opportunity.Opportunity, _ *CapabilityProfile) (*ScoreResult, error) {
	return m.result, m.err
}

// --- Recommendation constants ---

// TestRecommendation_Values verifies the string values of recommendation constants.
func TestRecommendation_Values(t *testing.T) {
	tests := []struct {
		rec Recommendation
		str string
	}{
		{RecommendationBid, "BID"},
		{RecommendationNoBid, "NO_BID"},
		{RecommendationReview, "REVIEW"},
	}
	for _, tt := range tests {
		if string(tt.rec) != tt.str {
			t.Errorf("Expected %q, got %q", tt.str, string(tt.rec))
		}
	}
}

// --- Signal computation ---

// TestComputeSignals_PrimaryNAICSMatch verifies primary NAICS match detection.
func TestComputeSignals_PrimaryNAICSMatch(t *testing.T) {
	opp := testOpportunity()
	opp.NAICSCode = "541512" // in PrimaryNAICS

	sig := computeSignals(opp, testProfile())

	if !sig.primaryNAICSMatch {
		t.Error("Expected primaryNAICSMatch=true for code in PrimaryNAICS")
	}
	if sig.secondaryNAICSMatch {
		t.Error("Expected secondaryNAICSMatch=false when primary already matched")
	}
}

// TestComputeSignals_SecondaryNAICSMatch verifies secondary NAICS match when primary misses.
func TestComputeSignals_SecondaryNAICSMatch(t *testing.T) {
	opp := testOpportunity()
	opp.NAICSCode = "541511" // in SecondaryNAICS

	sig := computeSignals(opp, testProfile())

	if sig.primaryNAICSMatch {
		t.Error("Expected primaryNAICSMatch=false")
	}
	if !sig.secondaryNAICSMatch {
		t.Error("Expected secondaryNAICSMatch=true for code in SecondaryNAICS")
	}
}

// TestComputeSignals_NoNAICSMatch verifies no match when code is absent from both lists.
func TestComputeSignals_NoNAICSMatch(t *testing.T) {
	opp := testOpportunity()
	opp.NAICSCode = "999999"

	sig := computeSignals(opp, testProfile())

	if sig.primaryNAICSMatch || sig.secondaryNAICSMatch {
		t.Error("Expected no NAICS match for unknown code")
	}
}

// TestComputeSignals_CompetencyTagOverlap verifies competency matching is case-insensitive.
func TestComputeSignals_CompetencyTagOverlap(t *testing.T) {
	opp := testOpportunity()
	opp.Description = "Cloud migration with Kubernetes and DevOps pipeline automation."

	sig := computeSignals(opp, testProfile())

	if len(sig.matchingTags) == 0 {
		t.Error("Expected at least one competency tag match")
	}
	// Verify specific tags were found
	tagSet := make(map[string]bool)
	for _, tag := range sig.matchingTags {
		tagSet[tag] = true
	}
	for _, expected := range []string{"cloud", "Kubernetes", "DevOps"} {
		if !tagSet[expected] {
			t.Errorf("Expected tag %q in matching tags, got %v", expected, sig.matchingTags)
		}
	}
}

// TestComputeSignals_NoCompetencyOverlap verifies empty tags when description is unrelated.
func TestComputeSignals_NoCompetencyOverlap(t *testing.T) {
	opp := testOpportunity()
	opp.Title = "Janitorial services"
	opp.Description = "Office cleaning, floor waxing, and window washing for federal property."

	sig := computeSignals(opp, testProfile())

	if len(sig.matchingTags) != 0 {
		t.Errorf("Expected no competency matches, got: %v", sig.matchingTags)
	}
}

// TestComputeSignals_SDBApplicable verifies SDB factor triggers on matching set-aside.
func TestComputeSignals_SDBApplicable(t *testing.T) {
	opp := testOpportunity()
	opp.SetAsideCode = "SBA"

	sig := computeSignals(opp, testProfile())

	if !sig.sdbApplicable {
		t.Error("Expected sdbApplicable=true when set-aside matches and company is SDB")
	}
}

// TestComputeSignals_SDBNotApplicable verifies SDB factor absent when set-aside does not match.
func TestComputeSignals_SDBNotApplicable(t *testing.T) {
	tests := []struct {
		name         string
		setAsideCode string
	}{
		{"non-qualifying code", "WOSB"},
		{"empty set-aside", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opp := testOpportunity()
			opp.SetAsideCode = tt.setAsideCode

			sig := computeSignals(opp, testProfile())

			if sig.sdbApplicable {
				t.Errorf("Expected sdbApplicable=false for set-aside %q", tt.setAsideCode)
			}
		})
	}
}

// TestComputeSignals_SDBFalse verifies SDB factor absent when company is not SDB.
func TestComputeSignals_SDBFalse(t *testing.T) {
	profile := testProfile()
	profile.IsSDB = false
	opp := testOpportunity()
	opp.SetAsideCode = "SBA"

	sig := computeSignals(opp, profile)

	if sig.sdbApplicable {
		t.Error("Expected sdbApplicable=false when company is not SDB")
	}
}

// TestComputeSignals_PastPerformance verifies past performance keyword matching.
func TestComputeSignals_PastPerformance(t *testing.T) {
	opp := testOpportunity()
	opp.Description = "GSA requires cloud migration and infrastructure modernization services."

	sig := computeSignals(opp, testProfile())

	if len(sig.pastPerfKeywords) == 0 {
		t.Error("Expected past performance keyword matches")
	}
}

// --- Prompt building ---

// TestBuildScoringPrompt_ContainsRequiredElements verifies prompt contains critical context.
func TestBuildScoringPrompt_ContainsRequiredElements(t *testing.T) {
	opp := testOpportunity()
	profile := testProfile()
	sig := computeSignals(opp, profile)

	prompt := buildScoringPrompt(opp, profile, sig)

	required := []string{
		profile.CompanyName,
		opp.Title,
		opp.Agency,
		opp.NAICSCode,
		"BID",
		"NO_BID",
		"REVIEW",
		"score",
		"reasoning",
		"requirements",
	}
	for _, element := range required {
		if !strings.Contains(prompt, element) {
			t.Errorf("Prompt missing required element: %q", element)
		}
	}
}

// TestBuildScoringPrompt_SignalsReflected verifies signal flags appear in prompt.
func TestBuildScoringPrompt_SignalsReflected(t *testing.T) {
	opp := testOpportunity()
	opp.NAICSCode = "541512" // primary match
	profile := testProfile()
	sig := computeSignals(opp, profile)

	prompt := buildScoringPrompt(opp, profile, sig)

	if !strings.Contains(prompt, "PRIMARY NAICS match") {
		t.Error("Prompt should reflect primary NAICS match signal")
	}
}

// --- Response validation ---

// TestValidateAndConvert_ValidBID verifies successful BID response conversion.
func TestValidateAndConvert_ValidBID(t *testing.T) {
	raw := &scoreResponse{
		Score:          82,
		Recommendation: "BID",
		Reasoning:      "Strong primary NAICS match (541512) and multiple competency overlaps (cloud, GCP, DevOps). SBA set-aside advantage applies.",
		Requirements:   []string{"GCP cloud expertise", "DevOps automation experience", "Active security clearance"},
	}

	result, err := validateAndConvert(raw)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Score != 82 {
		t.Errorf("Score: got %d, want 82", result.Score)
	}
	if result.Recommendation != RecommendationBid {
		t.Errorf("Recommendation: got %q, want BID", result.Recommendation)
	}
	if result.Reasoning == "" {
		t.Error("Reasoning must not be empty")
	}
	if len(result.Requirements) != 3 {
		t.Errorf("Requirements: got %d, want 3", len(result.Requirements))
	}
}

// TestValidateAndConvert_ValidNoBid verifies NO_BID response.
func TestValidateAndConvert_ValidNoBid(t *testing.T) {
	raw := &scoreResponse{
		Score:          25,
		Recommendation: "NO_BID",
		Reasoning:      "No NAICS match and no competency overlap. This opportunity requires specialized clearances not held.",
		Requirements:   []string{"TS/SCI clearance"},
	}
	result, err := validateAndConvert(raw)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Recommendation != RecommendationNoBid {
		t.Errorf("Expected NO_BID, got %q", result.Recommendation)
	}
}

// TestValidateAndConvert_ValidReview verifies REVIEW response.
func TestValidateAndConvert_ValidReview(t *testing.T) {
	raw := &scoreResponse{
		Score:          55,
		Recommendation: "REVIEW",
		Reasoning:      "Secondary NAICS match. Some competency overlap but missing key requirements.",
		Requirements:   []string{"OSCP certification", "FedRAMP experience"},
	}
	result, err := validateAndConvert(raw)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Recommendation != RecommendationReview {
		t.Errorf("Expected REVIEW, got %q", result.Recommendation)
	}
}

// TestValidateAndConvert_ScoreBoundaries verifies boundary scores 0 and 100 are valid.
func TestValidateAndConvert_ScoreBoundaries(t *testing.T) {
	for _, score := range []int{0, 100} {
		raw := &scoreResponse{
			Score:          score,
			Recommendation: "NO_BID",
			Reasoning:      "Boundary test.",
			Requirements:   []string{},
		}
		if _, err := validateAndConvert(raw); err != nil {
			t.Errorf("Score %d should be valid, got error: %v", score, err)
		}
	}
}

// TestValidateAndConvert_InvalidScore verifies scores outside 0–100 are rejected.
func TestValidateAndConvert_InvalidScore(t *testing.T) {
	for _, score := range []int{-1, 101, -100, 200} {
		raw := &scoreResponse{
			Score:          score,
			Recommendation: "BID",
			Reasoning:      "Test",
			Requirements:   []string{},
		}
		if _, err := validateAndConvert(raw); err == nil {
			t.Errorf("Score %d should be invalid", score)
		}
	}
}

// TestValidateAndConvert_InvalidRecommendation verifies unknown recommendations are rejected.
func TestValidateAndConvert_InvalidRecommendation(t *testing.T) {
	for _, rec := range []string{"MAYBE", "YES", "NO", "", "bid"} {
		raw := &scoreResponse{
			Score:          50,
			Recommendation: rec,
			Reasoning:      "Test",
			Requirements:   []string{},
		}
		if _, err := validateAndConvert(raw); err == nil {
			t.Errorf("Recommendation %q should be invalid", rec)
		}
	}
}

// TestValidateAndConvert_EmptyReasoning verifies empty reasoning is rejected.
func TestValidateAndConvert_EmptyReasoning(t *testing.T) {
	for _, reasoning := range []string{"", "   ", "\t\n"} {
		raw := &scoreResponse{
			Score:          50,
			Recommendation: "REVIEW",
			Reasoning:      reasoning,
			Requirements:   []string{},
		}
		if _, err := validateAndConvert(raw); err == nil {
			t.Errorf("Empty reasoning %q should be rejected", reasoning)
		}
	}
}

// TestValidateAndConvert_NilRequirementsBecomesEmptySlice verifies nil → empty slice.
func TestValidateAndConvert_NilRequirementsBecomesEmptySlice(t *testing.T) {
	raw := &scoreResponse{
		Score:          60,
		Recommendation: "REVIEW",
		Reasoning:      "Moderate fit with some gaps.",
		Requirements:   nil,
	}
	result, err := validateAndConvert(raw)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Requirements == nil {
		t.Error("Requirements must be non-nil (use empty slice)")
	}
}

// --- ScoreAndSave integration ---

// TestScoreAndSave_WritesEnrichedOpportunity verifies full write-back to store.
func TestScoreAndSave_WritesEnrichedOpportunity(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	s, err := store.NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	opp := testOpportunity()
	if err := s.Save(ctx, opp); err != nil {
		t.Fatalf("Failed to pre-save opportunity: %v", err)
	}

	mock := &mockScorer{
		result: &ScoreResult{
			Score:          82,
			Recommendation: RecommendationBid,
			Reasoning:      "Strong primary NAICS match (541512). Multiple competency overlaps: cloud, GCP, Kubernetes, DevOps. SBA advantage applies.",
			Requirements:   []string{"GCP expertise", "Kubernetes experience", "DevOps automation"},
		},
	}

	result, err := ScoreAndSave(ctx, mock, s, opp, testProfile())
	if err != nil {
		t.Fatalf("ScoreAndSave failed: %v", err)
	}

	// Verify returned result
	if result.Score != 82 {
		t.Errorf("Result score: got %d, want 82", result.Score)
	}
	if result.Recommendation != RecommendationBid {
		t.Errorf("Result recommendation: got %q, want BID", result.Recommendation)
	}
	if result.Reasoning == "" {
		t.Error("Result reasoning must not be empty")
	}

	// Verify the stored opportunity reflects all scored fields
	stored, err := s.Get(ctx, opp.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve stored opportunity: %v", err)
	}

	const wantScore = 0.82
	if stored.Score != wantScore {
		t.Errorf("Stored Score: got %f, want %f", stored.Score, wantScore)
	}
	if stored.Recommendation != "BID" {
		t.Errorf("Stored Recommendation: got %q, want BID", stored.Recommendation)
	}
	if stored.ScoreReasoning == "" {
		t.Error("Stored ScoreReasoning must not be empty")
	}
	if stored.ScoredAt == nil {
		t.Error("Stored ScoredAt must be set")
	}
	if len(stored.Requirements) == 0 {
		t.Error("Stored Requirements must not be empty")
	}
}

// TestScoreAndSave_NilOpportunity verifies error for nil opportunity.
func TestScoreAndSave_NilOpportunity(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	s, err := store.NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	mock := &mockScorer{result: &ScoreResult{Score: 50, Recommendation: RecommendationReview, Reasoning: "test", Requirements: []string{}}}
	_, err = ScoreAndSave(ctx, mock, s, nil, testProfile())
	if err == nil {
		t.Error("Expected error for nil opportunity")
	}
}

// TestScoreAndSave_NilProfile verifies error for nil profile.
func TestScoreAndSave_NilProfile(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	s, err := store.NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	mock := &mockScorer{result: &ScoreResult{Score: 50, Recommendation: RecommendationReview, Reasoning: "test", Requirements: []string{}}}
	_, err = ScoreAndSave(ctx, mock, s, testOpportunity(), nil)
	if err == nil {
		t.Error("Expected error for nil profile")
	}
}

// TestScoreAndSave_ScorerError propagates the scorer error.
func TestScoreAndSave_ScorerError(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	s, err := store.NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	opp := testOpportunity()
	mock := &mockScorer{err: fmt.Errorf("LLM unavailable")}

	_, err = ScoreAndSave(ctx, mock, s, opp, testProfile())
	if err == nil {
		t.Error("Expected error to propagate from scorer")
	}
	if !strings.Contains(err.Error(), "LLM unavailable") {
		t.Errorf("Error should contain underlying cause, got: %v", err)
	}
}

// TestScoreAndSave_UpdatesTimestamps verifies ScoredAt and UpdatedAt are set.
func TestScoreAndSave_UpdatesTimestamps(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	s, err := store.NewJSONStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	opp := testOpportunity()
	if err := s.Save(ctx, opp); err != nil {
		t.Fatalf("Failed to pre-save: %v", err)
	}

	mock := &mockScorer{
		result: &ScoreResult{
			Score: 70, Recommendation: RecommendationBid,
			Reasoning: "Good fit.", Requirements: []string{},
		},
	}

	_, err = ScoreAndSave(ctx, mock, s, opp, testProfile())
	if err != nil {
		t.Fatalf("ScoreAndSave failed: %v", err)
	}

	stored, err := s.Get(ctx, opp.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve: %v", err)
	}

	if stored.ScoredAt == nil {
		t.Error("ScoredAt must be set after scoring")
	}
	if stored.UpdatedAt.IsZero() {
		t.Error("UpdatedAt must be set after scoring")
	}
}
