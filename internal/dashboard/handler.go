package dashboard

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/Mawar2/Kaimi/internal/opportunity"
)

// Handler wraps the dashboard service and manages HTTP routing.
type Handler struct {
	svc          *Service
	mux          *http.ServeMux
	tmpl         *template.Template
	detailTmpl   *template.Template
	notFoundTmpl *template.Template
	Now          func() time.Time
}

// NewHandler initializes a new dashboard handler.
func NewHandler(svc *Service) *Handler {
	h := &Handler{
		svc: svc,
		mux: http.NewServeMux(),
		Now: time.Now,
	}
	h.setupRoutes()
	h.setupTemplates()
	return h
}

func (h *Handler) setupRoutes() {
	h.mux.HandleFunc("/", h.handleList)
	h.mux.HandleFunc("GET /opportunity/{id}", h.handleDetail)
}

func (h *Handler) setupTemplates() {
	// For now, we'll use a string template for simplicity,
	// but in a real project we'd load from files.
	// We'll follow the ux-spec.md for the layout.
	const layoutTmpl = `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta http-equiv="refresh" content="30">
  <title>Kaimi — {{.PageTitle}}</title>
  {{faviconLink}}
  {{styleTag}}
  <style>
    /* Page-specific rules only; all visual values come from the design-system
       tokens emitted by styleTag (docs/dashboard/ux-spec.md, issue #141). */
    body { margin: 1rem 2rem; }
    table { border-collapse: collapse; width: 100%; background: var(--surface); }
    th, td { border: 1px solid var(--border); padding: 0.4rem 0.6rem; text-align: left; }
    th { background: var(--surface-2); }
    tr:nth-child(even) { background: var(--surface-2); }
    .stage-cards { display: flex; gap: 1rem; flex-wrap: wrap; margin-bottom: 1.5rem; }
    .stage-card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--r-sm); padding: 0.75rem 1rem; min-width: 120px; }
    .stage-card .count { font-size: 2rem; font-weight: bold; font-family: var(--font-mono); }
    .stage-card-alert { background: var(--st-human-bg); border-color: var(--st-human); }
    .deadline-soon { background: var(--st-failed-bg); color: var(--st-failed); font-weight: bold; }
    .filter-bar { margin-bottom: 0.75rem; font-size: 0.9rem; color: var(--ink-3); }
    a { color: var(--primary); }
  </style>
</head>
<body>
  {{headerLockup}}
  <div class="stage-cards">
    {{range .Cards}}
    <div class="stage-card {{if .Alert}}stage-card-alert{{end}}">
      <div class="label">{{.Label}}</div>
      <div class="count">{{.Count}}</div>
    </div>
    {{end}}
  </div>

  <div class="filter-bar">
    <form method="GET" action="/opportunities">
      <label for="stage">Stage:</label>
      <select name="stage" id="stage">
        <option value="">All</option>
        <option value="Hunted" {{if eq .ActiveStage "Hunted"}}selected{{end}}>Hunted</option>
        <option value="Scored" {{if eq .ActiveStage "Scored"}}selected{{end}}>Scored</option>
        <option value="Selected" {{if eq .ActiveStage "Selected"}}selected{{end}}>Selected</option>
        <option value="In Proposal" {{if eq .ActiveStage "In Proposal"}}selected{{end}}>In Proposal</option>
        <option value="Awaiting Human Review" {{if eq .ActiveStage "Awaiting Human Review"}}selected{{end}}>Awaiting Human Review</option>
        <option value="Finalized" {{if eq .ActiveStage "Finalized"}}selected{{end}}>Finalized</option>
      </select>

      <label for="minScore">Min Score:</label>
      <input type="number" name="minScore" id="minScore" step="0.1" min="0" max="1" value="{{.ActiveMinScore}}">

      <label for="sort">Sort:</label>
      <select name="sort" id="sort">
        <option value="deadline" {{if eq .ActiveSort "deadline"}}selected{{end}}>Deadline</option>
        <option value="score" {{if eq .ActiveSort "score"}}selected{{end}}>Score</option>
      </select>

      <button type="submit">Apply</button>
      <a href="/opportunities">Clear</a>
    </form>
  </div>

  <table>
    <thead>
      <tr>
        <th>ID</th>
        <th>Title</th>
        <th>Agency</th>
        <th>NAICS</th>
        <th>Score</th>
        <th>Stage</th>
        <th>Deadline</th>
        <th>Last Updated</th>
      </tr>
    </thead>
    <tbody>
      {{range .Rows}}
      <tr class="{{if .DeadlineSoon}}deadline-soon{{end}}">
        <td>{{.ID}}</td>
        <td><a href="/opportunity/{{.ID}}">{{.Title}}</a></td>
        <td>{{.Agency}}</td>
        <td>{{.NAICSCode}}</td>
        <td>
          {{if gt .Score 0.0}}
            {{printf "%.1f" (multiply .Score 100)}}%
            <br><small>{{.ReasoningSnippet}}</small>
          {{else}}
            —
          {{end}}
        </td>
        <td>{{.Stage}}</td>
        <td>
          {{if .ResponseDeadline.IsZero}}—{{else}}{{.ResponseDeadline.Format "2006-01-02"}}{{end}}
        </td>
        <td>{{.LastUpdated.Format "2006-01-02 15:04"}}</td>
      </tr>
      {{else}}
      <tr>
        <td colspan="8">No opportunities found.</td>
      </tr>
      {{end}}
    </tbody>
  </table>
</body>
</html>
`
	// detailTmpl renders /opportunity/{id} per docs/dashboard/ux-spec.md View 2,
	// composing the design-system renderers (issue #111).
	const detailTmpl = `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta http-equiv="refresh" content="30">
  <title>Kaimi — {{.PageTitle}}</title>
  {{faviconLink}}
  {{styleTag}}
  <style>
    /* Page-specific rules only; visual values come from the design-system
       tokens emitted by styleTag (docs/dashboard/ux-spec.md). */
    body { margin: 1rem 2rem; max-width: 920px; }
    .head { display: flex; align-items: center; gap: 1rem; margin: 0.75rem 0 0.5rem; flex-wrap: wrap; }
    .head h1 { font: var(--t-h2); }
    .metaline { display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; margin-bottom: 1rem; color: var(--ink-2); }
    h2 { font: var(--t-h3); margin: 1.5rem 0 0.5rem; }
    table.kv { border-collapse: collapse; width: 100%; background: var(--surface); }
    table.kv td { border: 1px solid var(--border); padding: 0.35rem 0.6rem; vertical-align: top; }
    table.kv td:first-child { color: var(--ink-3); width: 200px; background: var(--surface-2); }
    table.kv ul { margin: 0; padding-left: 1.1rem; }
    pre { white-space: pre-wrap; background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--r-sm); padding: 0.75rem; }
    .deadline-soon { background: var(--st-failed-bg); color: var(--st-failed); font-weight: bold; }
    a { color: var(--primary); }
  </style>
</head>
<body>
  {{headerLockup}}
  <a href="/">← Back to pipeline</a>
  <div class="head">
    {{if gt .Opp.Score 0.0}}{{fitRing .Opp.Score}}{{end}}
    <h1>{{.Opp.Title}}</h1>
  </div>
  <div class="metaline">
    <span>{{.Opp.Agency}}</span>
    {{if .Opp.NAICSCode}}{{metaTag (printf "NAICS %s" .Opp.NAICSCode)}}{{end}}
    {{if .Opp.SolicitationNum}}{{metaTag (printf "SOL# %s" .Opp.SolicitationNum)}}{{end}}
    {{recPill .Opp.Recommendation}}
    {{if .DeadlineStr}}{{deadlinePill .DeadlineStr .DeadlineDays}}{{end}}
  </div>

  <h2>Identification</h2>
  <table class="kv">
    <tr><td>ID</td><td>{{.Opp.ID}}</td></tr>
    <tr><td>Title</td><td>{{.Opp.Title}}</td></tr>
    <tr><td>Solicitation #</td><td>{{orDash .Opp.SolicitationNum}}</td></tr>
    <tr><td>Agency</td><td>{{orDash .Opp.Agency}}</td></tr>
    <tr><td>Office</td><td>{{orDash .Opp.Office}}</td></tr>
    <tr><td>Type</td><td>{{orDash .Opp.Type}}</td></tr>
    <tr><td>Contract Type</td><td>{{orDash .Opp.ContractType}}</td></tr>
    <tr><td>Set-Aside</td><td>{{orDash .Opp.SetAsideCode}}</td></tr>
    <tr><td>Place of Performance</td><td>{{orDash .Opp.PlaceOfPerformance}}</td></tr>
    <tr><td>SAM.gov Link</td><td>{{if .Opp.URL}}<a href="{{.Opp.URL}}">View on SAM.gov</a>{{else}}&mdash;{{end}}</td></tr>
  </table>

  <h2>Dates</h2>
  <table class="kv">
    <tr><td>Posted</td><td>{{orDash .PostedDateStr}}</td></tr>
    <tr><td>Response Deadline</td><td{{if .DeadlineSoon}} class="deadline-soon"{{end}}>{{orDash .DeadlineStr}}{{if .DeadlineSoon}} ⚠{{end}}</td></tr>
    <tr><td>Created (local record)</td><td>{{orDash .CreatedAtStr}}</td></tr>
    <tr><td>Last Updated</td><td>{{orDash .UpdatedAtStr}}</td></tr>
  </table>

  <h2>Classification</h2>
  <table class="kv">
    <tr><td>NAICS Code</td><td>{{orDash .Opp.NAICSCode}}</td></tr>
    <tr><td>NAICS Description</td><td>{{orDash .Opp.NAICSDescription}}</td></tr>
  </table>

  <h2>Description</h2>
  {{if .Opp.Description}}<pre>{{.Opp.Description}}</pre>{{else}}<p>&mdash;</p>{{end}}

  <h2>Scoring</h2>
  <table class="kv">
    <tr><td>Score</td><td>{{.ScoreDisplay}}</td></tr>
    <tr><td>Recommendation</td><td>{{if .Opp.Recommendation}}{{recPill .Opp.Recommendation}}{{else}}&mdash;{{end}}</td></tr>
    <tr><td>Scored At</td><td>{{orDash .ScoredAtStr}}</td></tr>
    <tr><td>Requirements</td><td>{{if .Opp.Requirements}}<ul>{{range .Opp.Requirements}}<li>{{.}}</li>{{end}}</ul>{{else}}&mdash;{{end}}</td></tr>
    <tr><td>Full Reasoning</td><td>{{if .Opp.ScoreReasoning}}<pre>{{.Opp.ScoreReasoning}}</pre>{{else}}&mdash;{{end}}</td></tr>
  </table>

  <h2>Eligibility</h2>
  <div id="eligibility-placeholder">Eligibility check: not yet implemented (Phase 1+)</div>

  <h2>Proposal Status</h2>
  <table class="kv">
    <tr><td>Current Stage</td><td>{{.DerivedStage}}</td></tr>
    <tr><td>Selected</td><td>{{if .Opp.Selected}}Yes{{else}}No{{end}}</td></tr>
    <tr><td>Selected At</td><td>{{orDash .SelectedAtStr}}</td></tr>
    <tr><td>Proposal Status</td><td>{{orDash .Opp.ProposalStatus}}</td></tr>
  </table>
</body>
</html>
`

	// notFoundTmpl is the plain 404 page; it deliberately omits the
	// auto-refresh meta tag (ux-spec: nothing new to fetch).
	const notFoundTmpl = `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Kaimi — Not Found</title>
  {{faviconLink}}
  {{styleTag}}
  <style>body { margin: 1rem 2rem; } a { color: var(--primary); }</style>
</head>
<body>
  {{headerLockup}}
  <p>Opportunity not found: {{.ID}}</p>
  <p><a href="/">← Back to pipeline</a></p>
</body>
</html>
`

	funcMap := template.FuncMap{
		"multiply": func(a, b float64) float64 { return a * b },
		// Brand and design-system assets (issues #126/#132/#141): the layout
		// composes these instead of hardcoding visual values.
		"faviconLink":  FaviconLink,
		"styleTag":     StyleTag,
		"headerLockup": HeaderLockup,
		// Design-system component renderers (issue #111).
		"fitRing": func(score float64) template.HTML {
			// Opportunity.Score is 0.0-1.0; the ring takes 0-100.
			return FitRing(int(math.Round(score*100)), 64)
		},
		"recPill":      RecommendationPill,
		"deadlinePill": DeadlinePill,
		"metaTag":      MetaTag,
		"orDash": func(s string) string {
			if s == "" {
				return "—"
			}
			return s
		},
	}
	h.tmpl = template.Must(template.New("layout").Funcs(funcMap).Parse(layoutTmpl))
	h.detailTmpl = template.Must(template.New("detail").Funcs(funcMap).Parse(detailTmpl))
	h.notFoundTmpl = template.Must(template.New("notfound").Funcs(funcMap).Parse(notFoundTmpl))
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	query := r.URL.Query()

	opts := ListOptions{
		Now: h.Now(),
	}

	if s := query.Get("stage"); s != "" {
		st := Stage(s)
		opts.Stage = &st
	}

	if ms := query.Get("minScore"); ms != "" {
		if f, err := strconv.ParseFloat(ms, 64); err == nil {
			opts.MinScore = f
		}
	}

	if sort := query.Get("sort"); sort != "" {
		opts.SortBy = SortKey(sort)
	}

	rows, err := h.svc.List(ctx, opts)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to list opportunities: %v", err), http.StatusInternalServerError)
		return
	}

	// For stage cards, we need the counts across all stages (ignoring filters)
	counts, err := h.svc.CountsByStage(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load counts for cards: %v", err), http.StatusInternalServerError)
		return
	}

	data := OverviewData{
		PageTitle: "Overview",
		Cards: []StageCard{
			{Label: "Hunted", Stage: string(StageHunted), Count: counts[StageHunted]},
			{Label: "Scored", Stage: string(StageScored), Count: counts[StageScored]},
			{Label: "Selected", Stage: string(StageSelected), Count: counts[StageSelected]},
			{Label: "In Proposal", Stage: string(StageInProposal), Count: counts[StageInProposal]},
			{Label: "Awaiting Review", Stage: string(StageAwaitingHumanReview), Count: counts[StageAwaitingHumanReview], Alert: counts[StageAwaitingHumanReview] > 0},
			{Label: "Finalized", Stage: string(StageFinalized), Count: counts[StageFinalized]},
		},
		Rows:           rows,
		ActiveStage:    query.Get("stage"),
		ActiveMinScore: query.Get("minScore"),
		ActiveSort:     string(opts.SortBy),
	}

	if data.ActiveSort == "" {
		data.ActiveSort = "deadline"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.Execute(w, data); err != nil {
		// Log error and return internal server error if possible
		fmt.Printf("template execution failed: %v\n", err)
	}
}

// OverviewData matches the contract in ux-spec.md
type OverviewData struct {
	PageTitle      string
	Cards          []StageCard
	Rows           []OpportunityRow
	ActiveStage    string
	ActiveMinScore string
	ActiveSort     string
}

// StageCard represents a summary card for a pipeline stage.
type StageCard struct {
	Label string
	Stage string
	Count int
	Alert bool
}

// opportunityIDPattern is the conservative shape of a SAM.gov notice ID as
// used for store keys: alphanumeric start, then alphanumerics, dots, dashes,
// or underscores. Anything else (path traversal, spaces, markup) is rejected
// before the store is consulted.
var opportunityIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// DetailData is the view-model for the /opportunity/{id} page, per the
// contract in docs/dashboard/ux-spec.md.
type DetailData struct {
	PageTitle                                                             string
	Opp                                                                   *opportunity.Opportunity
	DerivedStage                                                          Stage
	ScoreDisplay                                                          string // "82.0%" or "—"
	DeadlineStr                                                           string // "2026-06-18" or "" when unset
	DeadlineDays                                                          int    // days until the deadline, for urgency styling
	DeadlineSoon                                                          bool
	PostedDateStr, CreatedAtStr, UpdatedAtStr, ScoredAtStr, SelectedAtStr string
}

func (h *Handler) handleDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !opportunityIDPattern.MatchString(id) {
		h.renderNotFound(w, id)
		return
	}

	opp, err := h.svc.Get(r.Context(), id)
	if err != nil {
		// The store returns an error for any missing id; with the id shape
		// already validated, render not-found rather than leaking internals.
		h.renderNotFound(w, id)
		return
	}

	now := h.Now()
	data := DetailData{
		PageTitle:     opp.Title,
		Opp:           opp,
		DerivedStage:  DeriveStage(opp),
		ScoreDisplay:  "—",
		DeadlineSoon:  isDeadlineSoon(opp.ResponseDeadline, now),
		PostedDateStr: fmtDate(opp.PostedDate),
		CreatedAtStr:  fmtDateTime(opp.CreatedAt),
		UpdatedAtStr:  fmtDateTime(opp.UpdatedAt),
		ScoredAtStr:   fmtDateTimePtr(opp.ScoredAt),
		SelectedAtStr: fmtDateTimePtr(opp.SelectedAt),
	}
	if opp.Score > 0 {
		data.ScoreDisplay = fmt.Sprintf("%.1f%%", opp.Score*100)
	}
	if !opp.ResponseDeadline.IsZero() {
		data.DeadlineStr = opp.ResponseDeadline.Format("2006-01-02")
		data.DeadlineDays = int(math.Ceil(opp.ResponseDeadline.Sub(now).Hours() / 24))
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.detailTmpl.Execute(w, data); err != nil {
		fmt.Printf("detail template execution failed: %v\n", err)
	}
}

// renderNotFound writes the 404 page with the (auto-escaped) id echoed back.
func (h *Handler) renderNotFound(w http.ResponseWriter, id string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if err := h.notFoundTmpl.Execute(w, struct{ ID string }{ID: id}); err != nil {
		fmt.Printf("not-found template execution failed: %v\n", err)
	}
}

// fmtDate formats a date-only field, returning "" for the zero value.
func fmtDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// fmtDateTime formats a timestamp field, returning "" for the zero value.
func fmtDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

// fmtDateTimePtr formats an optional timestamp, returning "" for nil.
func fmtDateTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return fmtDateTime(*t)
}
