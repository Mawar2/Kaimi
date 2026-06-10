package eval

import (
	"context"
	"fmt"
	"strings"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/scorer"
)

// Scorer is the consumer-side interface this harness needs from a bid/no-bid
// scorer. It is deliberately identical in shape to scorer.Scorer so the real
// *scorer.GeminiScorer satisfies it without any change to the scorer package; the
// fast test layer injects a mock with the same method.
type Scorer interface {
	Score(ctx context.Context, opp *opportunity.Opportunity, profile *scorer.CapabilityProfile) (*scorer.ScoreResult, error)
}

// ScorerCase is one labeled bid/no-bid evaluation case.
type ScorerCase struct {
	// Name identifies the case in the report. Required.
	Name string
	// Opportunity is the input the Scorer scores. Required (resolved from a
	// fixture reference when loaded from disk; see LoadScorerCases).
	Opportunity *opportunity.Opportunity
	// ExpectedRecommendation is the human-labeled ground-truth bid/no-bid call.
	ExpectedRecommendation scorer.Recommendation
	// ExpectedReasonKeywords are optional terms the scorer's reasoning should
	// mention (case-insensitive). Missing keywords are recorded per case but do
	// not affect the accuracy/precision/recall math.
	ExpectedReasonKeywords []string
}

// ScorerCaseResult is the per-case outcome of a scorer evaluation.
type ScorerCaseResult struct {
	Name      string                `json:"name"`
	Expected  scorer.Recommendation `json:"expected"`
	Predicted scorer.Recommendation `json:"predicted"`
	Correct   bool                  `json:"correct"`
	// MissingKeywords lists ExpectedReasonKeywords absent from the reasoning.
	MissingKeywords []string `json:"missing_keywords,omitempty"`
}

// ScorerReport is the structured reliability report for the Scorer.
//
// The confusion matrix and metrics treat BID as the positive class. REVIEW and
// NO_BID are both treated as "not BID" for the binary metrics, because the
// operational question the Scorer answers is "should a human spend time on this?"
// — only a BID asserts yes. Accuracy still credits an exact label match (so a
// predicted REVIEW against an expected REVIEW counts as correct).
type ScorerReport struct {
	Total          int                `json:"total"`
	TruePositives  int                `json:"true_positives"`
	FalsePositives int                `json:"false_positives"`
	TrueNegatives  int                `json:"true_negatives"`
	FalseNegatives int                `json:"false_negatives"`
	Accuracy       float64            `json:"accuracy"`
	Precision      float64            `json:"precision"`
	Recall         float64            `json:"recall"`
	Cases          []ScorerCaseResult `json:"cases"`
}

// EvaluateScorer runs the Scorer over every labeled case and returns a ScorerReport.
//
// It returns an error if the dataset is empty or the Scorer fails on any case —
// a scoring failure is a real reliability signal and must not be silently dropped.
func EvaluateScorer(ctx context.Context, s Scorer, profile *scorer.CapabilityProfile, cases []ScorerCase) (*ScorerReport, error) {
	if len(cases) == 0 {
		return nil, fmt.Errorf("eval: scorer dataset is empty")
	}

	rep := &ScorerReport{Total: len(cases), Cases: make([]ScorerCaseResult, 0, len(cases))}

	for _, c := range cases {
		if c.Opportunity == nil {
			return nil, fmt.Errorf("eval: scorer case %q has nil Opportunity", c.Name)
		}

		result, err := s.Score(ctx, c.Opportunity, profile)
		if err != nil {
			return nil, fmt.Errorf("eval: scorer failed on case %q: %w", c.Name, err)
		}

		predicted := result.Recommendation
		cr := ScorerCaseResult{
			Name:            c.Name,
			Expected:        c.ExpectedRecommendation,
			Predicted:       predicted,
			Correct:         predicted == c.ExpectedRecommendation,
			MissingKeywords: missingKeywords(result.Reasoning, c.ExpectedReasonKeywords),
		}
		rep.Cases = append(rep.Cases, cr)

		// Confusion matrix with BID as the positive class.
		expBID := c.ExpectedRecommendation == scorer.RecommendationBID
		predBID := predicted == scorer.RecommendationBID
		switch {
		case expBID && predBID:
			rep.TruePositives++
		case !expBID && predBID:
			rep.FalsePositives++
		case expBID && !predBID:
			rep.FalseNegatives++
		default:
			rep.TrueNegatives++
		}
	}

	correct := rep.TruePositives + rep.TrueNegatives
	rep.Accuracy = float64(correct) / float64(rep.Total)
	rep.Precision = ratio(rep.TruePositives, rep.TruePositives+rep.FalsePositives)
	rep.Recall = ratio(rep.TruePositives, rep.TruePositives+rep.FalseNegatives)

	return rep, nil
}

// ratio returns num/den, or 0 when den is 0 (no predicted/actual positives), so
// metrics never divide by zero on a degenerate dataset.
func ratio(num, den int) float64 {
	if den == 0 {
		return 0
	}
	return float64(num) / float64(den)
}

// missingKeywords returns the keywords not found (case-insensitive substring) in
// text. Returns nil when keywords is empty or all are present.
func missingKeywords(text string, keywords []string) []string {
	if len(keywords) == 0 {
		return nil
	}
	lower := strings.ToLower(text)
	var missing []string
	for _, kw := range keywords {
		if !strings.Contains(lower, strings.ToLower(kw)) {
			missing = append(missing, kw)
		}
	}
	return missing
}
