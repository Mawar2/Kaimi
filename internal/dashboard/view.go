package dashboard

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Mawar2/Kaimi/internal/opportunity"
	"github.com/Mawar2/Kaimi/internal/store"
)

// SortKey selects the sort field for List results.
type SortKey string

const (
	// SortByDeadline sorts by ResponseDeadline ascending (earliest deadline first).
	SortByDeadline SortKey = "deadline"
	// SortByScore sorts by Score descending (highest score first).
	SortByScore SortKey = "score"
)

// deadlineSoonWindow is the duration within which an upcoming deadline is flagged
// as "soon" on the OpportunityRow.
const deadlineSoonWindow = 7 * 24 * time.Hour

// ListOptions configures the behavior of Service.List.
type ListOptions struct {
	// Stage filters to a specific pipeline stage. nil returns all stages.
	Stage *Stage
	// MinScore filters to opportunities with Score >= MinScore. 0 means no filter.
	MinScore float64
	// SortBy selects the sort field. Defaults to SortByDeadline when zero.
	SortBy SortKey
	// Now is injected for DeadlineSoon computation. Zero value disables the flag.
	Now time.Time
}

// OpportunityRow is the view-model for a single row in the dashboard list view.
// It carries only the fields the list view needs; the full Opportunity is
// available via Service.Get for the detail page.
type OpportunityRow struct {
	// ID is the SAM.gov notice ID (store key).
	ID string
	// Title is the opportunity title.
	Title string
	// Agency is the issuing agency.
	Agency string
	// NAICSCode is the primary NAICS code.
	NAICSCode string
	// Score is the bid/no-bid score (0.0–1.0), zero if not yet scored.
	Score float64
	// ReasoningSnippet is the Scorer's reasoning text.
	ReasoningSnippet string
	// Stage is the derived pipeline stage.
	Stage Stage
	// ResponseDeadline is the proposal due date (zero if not set).
	ResponseDeadline time.Time
	// LastUpdated is the last store update timestamp.
	LastUpdated time.Time
	// DeadlineSoon is true when ResponseDeadline is upcoming and within 7 days of Now.
	DeadlineSoon bool
}

// Service provides read-only dashboard views over a store.Store.
// It never calls Store.Save or Store.Delete; it is safe to use with a read-only
// store proxy.
type Service struct {
	store store.Store
}

// NewService returns a Service backed by the given store.
func NewService(s store.Store) *Service {
	return &Service{store: s}
}

// List loads opportunities from the store, applies stage and score filters,
// and returns sorted OpportunityRows.
//
// Score filtering is delegated to store.Filter (store-side, efficient).
// Stage filtering is applied in Go after retrieval because Stage is derived
// from field values and is not a stored field.
func (svc *Service) List(ctx context.Context, opts ListOptions) ([]OpportunityRow, error) {
	opps, err := svc.store.List(ctx, &store.Filter{MinScore: opts.MinScore})
	if err != nil {
		return nil, fmt.Errorf("dashboard list: %w", err)
	}

	rows := make([]OpportunityRow, 0, len(opps))
	for _, opp := range opps {
		stage := DeriveStage(opp)
		if opts.Stage != nil && stage != *opts.Stage {
			continue
		}
		rows = append(rows, toRow(opp, stage, opts.Now))
	}

	switch opts.SortBy {
	case SortByScore:
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].Score > rows[j].Score
		})
	default: // SortByDeadline and zero value
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].ResponseDeadline.Before(rows[j].ResponseDeadline)
		})
	}

	return rows, nil
}

// CountsByStage returns a map of stage counts across all opportunities in the store.
func (svc *Service) CountsByStage(ctx context.Context) (map[Stage]int, error) {
	opps, err := svc.store.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("dashboard counts: %w", err)
	}
	return CountByStage(opps), nil
}

// Get returns the full Opportunity for the detail page.
// It reads through the store interface without mutation.
func (svc *Service) Get(ctx context.Context, id string) (*opportunity.Opportunity, error) {
	opp, err := svc.store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("dashboard get %s: %w", id, err)
	}
	return opp, nil
}

// toRow converts an Opportunity and its derived Stage to an OpportunityRow.
func toRow(opp *opportunity.Opportunity, stage Stage, now time.Time) OpportunityRow {
	return OpportunityRow{
		ID:               opp.ID,
		Title:            opp.Title,
		Agency:           opp.Agency,
		NAICSCode:        opp.NAICSCode,
		Score:            opp.Score,
		ReasoningSnippet: opp.ScoreReasoning,
		Stage:            stage,
		ResponseDeadline: opp.ResponseDeadline,
		LastUpdated:      opp.UpdatedAt,
		DeadlineSoon:     isDeadlineSoon(opp.ResponseDeadline, now),
	}
}

// isDeadlineSoon returns true when the deadline is upcoming and within
// deadlineSoonWindow (7 days) of now. Returns false when either time is zero,
// or when the deadline has already passed.
func isDeadlineSoon(deadline, now time.Time) bool {
	if now.IsZero() || deadline.IsZero() {
		return false
	}
	if deadline.Before(now) {
		return false
	}
	return deadline.Sub(now) <= deadlineSoonWindow
}
