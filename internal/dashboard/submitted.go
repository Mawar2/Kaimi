package dashboard

import (
	"fmt"
	"math"
	"net/http"
	"strings"
)

// SubmittedData is the view-model for the Submitted archive screen (the third
// nav destination — design handoff PIPELINE.md §3). It lists every proposal that
// has been submitted to SAM.gov, the dollar value of the pipeline, and the
// reference documents the agent team produced.
//
// Forward-compatible seam: the dollar stats and per-proposal award outcome
// require an EstimatedValue field on the Opportunity schema and a submission
// record (PIPELINE.md "Data model additions"). Those land with the build-loop's
// schema work; until then PendingValue/WonValue render "—" and every row shows
// the default "Pending award" outcome. See the TODO(phase-3) markers below.
type SubmittedData struct {
	shellData
	// Total is the all-time count of submitted proposals.
	Total int
	// PendingValue / WonValue are the $-awaiting-award / $-won headline stats.
	// TODO(phase-3): sum Opportunity.EstimatedValue once that field exists.
	PendingValue string
	WonValue     string
	// Query is the current search term (title / agency / SOL#).
	Query string
	// Cards are the submitted proposals matching the search.
	Cards []SubmittedCard
	// Empty is true when no submitted proposal matches.
	Empty bool
}

// SubmittedCard is one row in the Submitted archive.
type SubmittedCard struct {
	ID             string
	Title          string
	Agency         string
	SOL            string
	SubmittedLabel string
	ScorePct       int
	// StatusLabel is the award outcome. TODO(phase-3): real pending/won/lost
	// once the submission record persists outcomes; currently always "Pending
	// award".
	StatusLabel string
	// SolicitationURL is the SAM.gov opportunity page (opens in a new tab).
	SolicitationURL string
	// Docs are the reusable artifacts the team produced for this proposal.
	Docs []SubmittedDocRef
}

// SubmittedDocRef is one reference document on a submitted proposal.
type SubmittedDocRef struct {
	Name string
	// Href is where the document opens. Empty when the artifact is known to
	// exist but isn't independently served yet (TODO(phase-3): serve stored
	// draft/compliance/price volumes and GCS-backed solicitation files).
	Href string
}

// handleSubmitted renders the Submitted archive: submitted proposals, pipeline
// value stats, search, and per-proposal reference documents.
func (h *Handler) handleSubmitted(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := h.Now()
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	rows, err := h.svc.List(ctx, ListOptions{Now: now})
	if err != nil {
		fmt.Printf("submitted list failed: %v\n", err)
		http.Error(w, "failed to load submitted proposals", http.StatusInternalServerError)
		return
	}

	data := SubmittedData{
		shellData: shellData{PageTitle: "Submitted", ActiveNav: "submitted"},
		Query:     q,
		// TODO(phase-3): replace with formatted sums of Opportunity.EstimatedValue
		// over pending / won submissions once the schema field + outcome record exist.
		PendingValue: "—",
		WonValue:     "—",
	}

	ql := strings.ToLower(q)
	for i := range rows {
		row := &rows[i]
		if row.Stage != StageSubmitted {
			continue
		}
		// Stats and the nav badge count every submitted proposal; the search
		// only narrows the displayed list.
		data.Total++
		data.SubmittedCount++
		if ql != "" && !strings.Contains(strings.ToLower(row.Title+" "+row.Agency), ql) {
			continue
		}

		card := SubmittedCard{
			ID:          row.ID,
			Title:       row.Title,
			Agency:      row.Agency,
			ScorePct:    int(math.Round(row.Score * 100)),
			StatusLabel: "Pending award", // TODO(phase-3): real outcome
		}
		if !row.LastUpdated.IsZero() {
			card.SubmittedLabel = row.LastUpdated.Format("Jan 2, 2006")
		}
		// The full opportunity carries the solicitation number, URL and the
		// ingested solicitation documents — the archive's reference library.
		if opp, err := h.svc.Get(ctx, row.ID); err == nil {
			card.SOL = opp.SolicitationNum
			card.SolicitationURL = opp.URL
			// The produced draft + compliance result live on the workspace.
			card.Docs = append(card.Docs, SubmittedDocRef{
				Name: "Working draft & compliance",
				Href: "/workspace/" + opp.ID,
			})
			for _, d := range opp.Documents {
				card.Docs = append(card.Docs, SubmittedDocRef{Name: d.Filename, Href: d.SourceURL})
			}
		}
		data.Cards = append(data.Cards, card)
	}
	data.Empty = len(data.Cards) == 0

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.submittedTmpl.Execute(w, data); err != nil {
		fmt.Printf("submitted template execution failed: %v\n", err)
	}
}

// submittedContentTmpl is the Submitted archive screen. It reuses the existing
// design-system classes (.page/.stats/.pcard/.kbadge/.art-row) so it stays
// visually consistent with the rest of the app; the BD-report export and the
// per-row outcome control from PIPELINE.md are TODO(phase-3) follow-ups.
const submittedContentTmpl = `{{define "content"}}
<div class="page">
  <div class="page-head">
    <div class="eyebrow">Pipeline</div>
    <h1>Submitted</h1>
    <p class="lead">Every proposal that has gone out the door — what it is worth, and everything the team produced along the way.</p>
    <div class="stats">
      <div class="stat"><div class="v">{{.PendingValue}}<small> awaiting award</small></div></div>
      <div class="stat"><div class="v">{{.WonValue}}<small> won</small></div></div>
      <div class="stat"><div class="v">{{.Total}}<small> submitted</small></div></div>
    </div>
  </div>

  <form method="GET" action="/submitted" style="margin:18px 0">
    <input type="search" name="q" value="{{.Query}}" placeholder="Search by title or agency…"
      style="width:100%;max-width:420px;padding:9px 12px;border:1px solid rgba(16,30,60,0.12);border-radius:11px;font:inherit;background:#fff" />
  </form>

  {{if .Empty}}
  <div class="empty">
    <h3>Nothing in the archive yet</h3>
    <p>Submit a proposal and it lands here — with its value and every document the team produced.</p>
  </div>
  {{else}}
  <div class="sub-list" style="display:flex;flex-direction:column;gap:10px">
    {{range .Cards}}
    <div class="pcard" style="display:flex;flex-direction:column;gap:12px">
      <div style="display:flex;align-items:center;gap:14px">
        {{if .ScorePct}}{{fitRing .ScorePct 46}}{{end}}
        <div style="flex:1;min-width:0">
          <div style="font-weight:600;font-size:16px">{{.Title}}</div>
          <div style="color:#5A6B86;font-size:13px;margin-top:3px">
            {{.Agency}}{{if .SOL}} &middot; {{metaTag (printf "SOL# %s" .SOL)}}{{end}}{{if .SubmittedLabel}} &middot; Submitted {{.SubmittedLabel}}{{end}}
          </div>
        </div>
        <span class="kbadge">{{.StatusLabel}}</span>
      </div>
      <div class="art-row" style="display:flex;flex-wrap:wrap;gap:8px">
        {{if .SolicitationURL}}<a class="artifact2" href="{{.SolicitationURL}}" target="_blank" rel="noopener noreferrer">View solicitation</a>{{end}}
        {{range .Docs}}<a class="artifact2" href="{{if .Href}}{{.Href}}{{else}}#{{end}}">{{.Name}}</a>{{end}}
      </div>
    </div>
    {{end}}
  </div>
  {{end}}
</div>
{{end}}`
