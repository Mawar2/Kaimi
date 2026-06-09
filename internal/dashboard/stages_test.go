package dashboard

import (
	"testing"
	"time"

	"github.com/Mawar2/Kaimi/internal/opportunity"
)

// scored returns a pointer to a fixed time used to mark an opportunity as scored.
func scored() *time.Time {
	t := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return &t
}

func TestDeriveStage(t *testing.T) {
	tests := []struct {
		name string
		opp  opportunity.Opportunity
		want Stage
	}{
		// --- Hunted ---
		{
			name: "no score, not selected → Hunted",
			opp:  opportunity.Opportunity{},
			want: StageHunted,
		},
		{
			name: "ScoredAt nil, Selected false → Hunted",
			opp:  opportunity.Opportunity{Selected: false, ScoredAt: nil},
			want: StageHunted,
		},

		// --- Scored ---
		{
			name: "scored BID, not selected → Scored",
			opp:  opportunity.Opportunity{ScoredAt: scored(), Recommendation: "BID"},
			want: StageScored,
		},
		{
			name: "scored NO_BID, not selected → Scored",
			opp:  opportunity.Opportunity{ScoredAt: scored(), Recommendation: "NO_BID"},
			want: StageScored,
		},
		// Ambiguity: Recommendation=="REVIEW" is a sub-state badge within Scored,
		// NOT the same as StageAwaitingHumanReview.
		{
			name: "scored REVIEW recommendation, not selected → Scored (not AwaitingHumanReview)",
			opp:  opportunity.Opportunity{ScoredAt: scored(), Recommendation: "REVIEW"},
			want: StageScored,
		},
		// Ambiguity: ScoredAt set but Recommendation empty → Scored with "Unknown" sub-state.
		{
			name: "scored, empty recommendation → Scored",
			opp:  opportunity.Opportunity{ScoredAt: scored(), Recommendation: ""},
			want: StageScored,
		},

		// --- Selected (selected, no ProposalStatus yet) ---
		{
			name: "selected, empty ProposalStatus → Selected",
			opp:  opportunity.Opportunity{Selected: true, ScoredAt: scored(), ProposalStatus: ""},
			want: StageSelected,
		},
		{
			name: "selected without prior scoring → Selected",
			opp:  opportunity.Opportunity{Selected: true, ProposalStatus: ""},
			want: StageSelected,
		},

		// --- In Proposal ---
		{
			name: "selected, outline in progress → InProposal",
			opp:  opportunity.Opportunity{Selected: true, ScoredAt: scored(), ProposalStatus: "outline:success"},
			want: StageInProposal,
		},
		{
			name: "selected, writer stage → InProposal",
			opp:  opportunity.Opportunity{Selected: true, ScoredAt: scored(), ProposalStatus: "writer:success"},
			want: StageInProposal,
		},
		// Ambiguity: :failed suffix is a system error, not a human-review request.
		{
			name: "selected, outline failed → InProposal (not AwaitingHumanReview)",
			opp:  opportunity.Opportunity{Selected: true, ScoredAt: scored(), ProposalStatus: "outline:failed"},
			want: StageInProposal,
		},
		{
			name: "selected, writer failed → InProposal",
			opp:  opportunity.Opportunity{Selected: true, ScoredAt: scored(), ProposalStatus: "writer:failed"},
			want: StageInProposal,
		},

		// --- Awaiting Human Review ---
		{
			name: "selected, outline needs_human → AwaitingHumanReview",
			opp:  opportunity.Opportunity{Selected: true, ScoredAt: scored(), ProposalStatus: "outline:needs_human"},
			want: StageAwaitingHumanReview,
		},
		{
			name: "selected, writer needs_human → AwaitingHumanReview",
			opp:  opportunity.Opportunity{Selected: true, ScoredAt: scored(), ProposalStatus: "writer:needs_human"},
			want: StageAwaitingHumanReview,
		},
		{
			name: "selected, final-review needs_human → AwaitingHumanReview",
			opp:  opportunity.Opportunity{Selected: true, ScoredAt: scored(), ProposalStatus: "final-review:needs_human"},
			want: StageAwaitingHumanReview,
		},

		// --- Finalized ---
		{
			name: "selected, final-review ready_to_submit → Finalized",
			opp:  opportunity.Opportunity{Selected: true, ScoredAt: scored(), ProposalStatus: "final-review:ready_to_submit"},
			want: StageFinalized,
		},

		// --- Ambiguity: Selected==false with non-empty ProposalStatus ---
		// Selected==false is authoritative; ProposalStatus is ignored (data anomaly).
		{
			name: "not selected but ProposalStatus set, ScoredAt set → Scored (Selected false wins)",
			opp:  opportunity.Opportunity{Selected: false, ScoredAt: scored(), ProposalStatus: "outline:success"},
			want: StageScored,
		},
		{
			name: "not selected but ProposalStatus set, ScoredAt nil → Hunted (Selected false wins)",
			opp:  opportunity.Opportunity{Selected: false, ScoredAt: nil, ProposalStatus: "outline:success"},
			want: StageHunted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveStage(&tc.opp)
			if got != tc.want {
				t.Errorf("DeriveStage(%+v) = %q, want %q", tc.opp, got, tc.want)
			}
		})
	}
}

func TestCountByStage(t *testing.T) {
	t1 := scored()

	opps := []opportunity.Opportunity{
		// 2 Hunted
		{ScoredAt: nil, Selected: false},
		{ScoredAt: nil, Selected: false},
		// 1 Scored
		{ScoredAt: t1, Selected: false, Recommendation: "BID"},
		// 1 Selected
		{Selected: true, ProposalStatus: ""},
		// 2 InProposal
		{Selected: true, ScoredAt: t1, ProposalStatus: "outline:success"},
		{Selected: true, ScoredAt: t1, ProposalStatus: "writer:failed"},
		// 1 AwaitingHumanReview
		{Selected: true, ScoredAt: t1, ProposalStatus: "outline:needs_human"},
		// 1 Finalized
		{Selected: true, ScoredAt: t1, ProposalStatus: "final-review:ready_to_submit"},
	}

	counts := CountByStage(opps)

	cases := []struct {
		stage Stage
		want  int
	}{
		{StageHunted, 2},
		{StageScored, 1},
		{StageSelected, 1},
		{StageInProposal, 2},
		{StageAwaitingHumanReview, 1},
		{StageFinalized, 1},
	}

	for _, c := range cases {
		if counts[c.stage] != c.want {
			t.Errorf("CountByStage[%q] = %d, want %d", c.stage, counts[c.stage], c.want)
		}
	}
}

func TestCountByStageEmpty(t *testing.T) {
	counts := CountByStage(nil)
	if len(counts) != 0 {
		t.Errorf("CountByStage(nil) = %v, want empty map", counts)
	}
}
