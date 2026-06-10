package dashboard

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"
)

// Handler wraps the dashboard service and manages HTTP routing.
type Handler struct {
	svc  *Service
	mux  *http.ServeMux
	tmpl *template.Template
	Now  func() time.Time
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
        <td>{{.Title}}</td>
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
	funcMap := template.FuncMap{
		"multiply": func(a, b float64) float64 { return a * b },
		// Brand and design-system assets (issues #126/#132/#141): the layout
		// composes these instead of hardcoding visual values.
		"faviconLink":  FaviconLink,
		"styleTag":     StyleTag,
		"headerLockup": HeaderLockup,
	}
	h.tmpl = template.Must(template.New("layout").Funcs(funcMap).Parse(layoutTmpl))
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
