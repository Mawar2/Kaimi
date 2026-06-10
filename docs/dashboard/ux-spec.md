# Dashboard UX Specification

**Last updated:** 2026-06-09
**Status:** Approved for implementation (Wave 3)

This document is the authoritative contract for the Kaimi dashboard's three views. Handler and template work must implement exactly what is described here — no more, no less. Any deviation from field names, query-param names, or layout requires updating this document first.

---

## Technology Constraints

- **Renderer:** Go stdlib `net/http` + `html/template`
- **Styling:** Minimal inline CSS only; no external CSS files, no JS framework, no external assets. Inline SVG and `data:` URIs (the brand mark and favicon from `internal/dashboard/brand.go`) count as inline, not external
- **JavaScript:** None. All interactivity is handled by HTML form submissions and server-side rendering
- **Auto-refresh:** `<meta http-equiv="refresh">` tag (see §Auto-Refresh below)
- **Auth:** None; localhost-only, trusted network assumed

---

## Auto-Refresh

Every page that shows live pipeline state includes a meta-refresh header:

```html
<meta http-equiv="refresh" content="30">
```

- **Interval:** 30 seconds
- **Scope:** Pipeline overview (`/`) and opportunity detail (`/opportunity/{id}`) both carry this tag
- **Why 30s:** Fast enough to see worker progress without overwhelming the server during active pipeline runs; slow enough that a human reading the detail page is not interrupted mid-sentence

---

## View 1 — Pipeline Overview (`/`)

The root handler (`GET /`) renders a single page with two sections: stage cards at the top, the opportunity table below.

### 1.1 Stage Cards

A horizontal row of cards, one per stage, showing the live count of opportunities in that stage.

| Card label | Stage name in data | Count source |
|---|---|---|
| **Hunted** | `hunted` | All opportunities with no score yet (`Score == 0` and `Recommendation == ""`) |
| **Scored — Bid** | `scored_bid` | `Recommendation == "BID"` |
| **Scored — No-Bid** | `scored_nobid` | `Recommendation == "NO_BID"` |
| **Selected** | `selected` | `Selected == true` and `ProposalStatus == ""` |
| **In Proposal** | `in_proposal` | `ProposalStatus` is one of `"outline"`, `"draft"` |
| **Awaiting Human Review** | `awaiting_review` | `ProposalStatus == "review"` |
| **Finalized** | `finalized` | `ProposalStatus == "finalized"` |

**Visual treatment:**
- Each card shows: label (bold), count (large number), no other content
- Cards are rendered as `<div>` elements in a flex row using inline `display:flex; gap:1rem`
- The "Awaiting Human Review" card uses the brand's "needs human" amber (`background:#FFF3E0; border:1px solid #E8870E`) to draw attention. Amber is reserved app-wide for "a human is needed" — never use it decoratively
- No links on the cards; they are informational only

### 1.2 Opportunity Table

Rendered below the stage cards on the same `/` page.

#### Columns (in display order)

| Column header | Field source | Notes |
|---|---|---|
| **ID** | `Opportunity.ID` | Truncated to 12 chars + `…` if longer; full value in `title` attribute |
| **Title** | `Opportunity.Title` | Links to `/opportunity/{id}` |
| **Agency** | `Opportunity.Agency` | Plain text |
| **NAICS** | `Opportunity.NAICSCode` | Shown as code only (e.g. `541511`) |
| **Score** | `Opportunity.Score` | Displayed as percentage `(Score × 100)%` with one decimal, e.g. `87.3%`; empty string `—` if `Score == 0` |
| **Reasoning** | `Opportunity.ScoreReasoning` | First 80 characters followed by `…` if longer; full text in `title` attribute; empty `—` if not yet scored |
| **Stage** | Derived from fields (see §Stage Derivation) | Human-readable label matching card labels |
| **Deadline** | `Opportunity.ResponseDeadline` | Formatted `2006-01-02`; see §Deadline Flagging |
| **Last Updated** | `Opportunity.UpdatedAt` | Formatted `2006-01-02 15:04` in server local time |

#### Stage Derivation (for table column and filtering)

The "stage" value is derived from `Opportunity` fields using this priority order:

```
if ProposalStatus == "finalized"                     → "finalized"
else if ProposalStatus == "review"                   → "awaiting_review"
else if ProposalStatus in {"outline","draft"}        → "in_proposal"
else if Selected == true                             → "selected"
else if Recommendation == "BID"                      → "scored_bid"
else if Recommendation == "NO_BID"                   → "scored_nobid"
else                                                 → "hunted"
```

This logic must be implemented in a single Go function (e.g. `deriveStage(o Opportunity) string`) shared between the card-count calculation and the table row renderer.

#### Deadline Flagging

If `ResponseDeadline` is within 7 calendar days from the current server time (inclusive of today), the deadline cell is rendered with a visual flag:

```html
<td style="background:#FCE8E8; color:#DC2626; font-weight:bold;">2026-06-14 ⚠</td>
```

If `ResponseDeadline` is zero (not set), display `—` with no flag.

---

## View 1 — Filters and Sort (Query Parameters)

All filters and sort controls on `/` are implemented as HTML `<form method="GET">` submissions. The query-param contract is:

| Param name | Accepted values | Default (when absent) | Description |
|---|---|---|---|
| `stage` | `hunted`, `scored_bid`, `scored_nobid`, `selected`, `in_proposal`, `awaiting_review`, `finalized`, or empty | _(show all)_ | Filter table to one stage only |
| `min_score` | Integer `0`–`100` (inclusive) | `0` | Hide rows where `Score × 100 < min_score` |
| `sort` | `deadline`, `score` | `deadline` | Sort order for the table |
| `order` | `asc`, `desc` | `asc` for `deadline`; `desc` for `score` | Secondary sort direction override |

**Rules:**
- Unknown param values are ignored (treated as default)
- `min_score` values outside `0`–`100` are clamped to the nearest bound
- Sort by `deadline` puts zero deadlines last
- Sort by `score` puts unscored opportunities (score `0`) last
- Multiple filters are combined with AND logic

**Active filter display:** When any filter is active, show a one-line summary above the table:

```
Showing: stage=scored_bid, min_score=60 | [Clear filters]
```

The "Clear filters" link points to `/` with no query params.

---

## View 2 — Opportunity Detail (`/opportunity/{id}`)

The detail handler (`GET /opportunity/{id}`) renders the full record for one opportunity.

### URL Pattern

- Path param: `{id}` corresponds to `Opportunity.ID`
- If `{id}` does not match any stored opportunity: render a plain 404 page with message `Opportunity not found: {id}`

### Sections and Fields

The detail page is divided into labeled sections using `<h2>` headings.

#### Section: Identification

| Label | Field |
|---|---|
| ID | `Opportunity.ID` |
| Title | `Opportunity.Title` |
| Solicitation # | `Opportunity.SolicitationNum` |
| Agency | `Opportunity.Agency` |
| Office | `Opportunity.Office` |
| Type | `Opportunity.Type` |
| Contract Type | `Opportunity.ContractType` |
| Set-Aside | `Opportunity.SetAsideCode` (empty → `—`) |
| Place of Performance | `Opportunity.PlaceOfPerformance` |
| SAM.gov Link | `Opportunity.URL` rendered as `<a href="...">View on SAM.gov</a>` |

#### Section: Dates

| Label | Field | Format |
|---|---|---|
| Posted | `Opportunity.PostedDate` | `2006-01-02` |
| Response Deadline | `Opportunity.ResponseDeadline` | `2006-01-02`; apply deadline flag (§Deadline Flagging) if within 7 days |
| Created (local record) | `Opportunity.CreatedAt` | `2006-01-02 15:04` |
| Last Updated | `Opportunity.UpdatedAt` | `2006-01-02 15:04` |

#### Section: Classification

| Label | Field |
|---|---|
| NAICS Code | `Opportunity.NAICSCode` |
| NAICS Description | `Opportunity.NAICSDescription` |

#### Section: Description

Full `Opportunity.Description` rendered inside a `<pre style="white-space:pre-wrap">` block to preserve line breaks. If empty: `—`.

#### Section: Scoring

| Label | Field | Format |
|---|---|---|
| Score | `Opportunity.Score` | `87.3%` (one decimal, `—` if zero) |
| Recommendation | `Opportunity.Recommendation` | `BID`, `NO_BID`, `REVIEW`, or `—`; `BID` shown in green, `NO_BID` in red |
| Scored At | `Opportunity.ScoredAt` | `2006-01-02 15:04` or `—` if nil |
| Requirements | `Opportunity.Requirements` | Unordered list `<ul>`; `—` if empty |
| Full Reasoning | `Opportunity.ScoreReasoning` | Rendered inside `<pre style="white-space:pre-wrap">`; `—` if empty |

#### Section: Eligibility

Eligibility results are derived from the set-aside code and the company's capability profile. In Phase 0 / Wave 3 this section is rendered as a placeholder:

```
Eligibility check: not yet implemented (Phase 1+)
```

The HTML element `<div id="eligibility-placeholder">` must be present so Wave 3 handler tests can assert its existence.

#### Section: Proposal Status

| Label | Field | Notes |
|---|---|---|
| Current Stage | Derived stage string (§Stage Derivation) | Human-readable |
| Selected | `Opportunity.Selected` | `Yes` / `No` |
| Selected At | `Opportunity.SelectedAt` | `2006-01-02 15:04` or `—` if nil |
| Proposal Status | `Opportunity.ProposalStatus` | Raw value or `—` if empty |

#### Navigation

A `← Back to pipeline` link at the top of the page pointing to `/` (no query params preserved, per simplicity principle).

---

## Shared Layout

Both views share a minimal outer layout, branded with the locked Kai wave system (see `internal/dashboard/brand.go`, implemented from the `Kaimi Brand.html` design handoff):

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta http-equiv="refresh" content="30">
  <title>Kaimi — {{.PageTitle}}</title>
  {{.FaviconLink}}   <!-- dashboard.FaviconLink(): inline data-URI brand favicon -->
  <style>
    body { font-family: system-ui, sans-serif; margin: 1rem 2rem; color: #0A1B3D; background: #FBFCFE; }
    table { border-collapse: collapse; width: 100%; }
    th, td { border: 1px solid rgba(16,30,60,0.12); padding: 0.4rem 0.6rem; text-align: left; }
    th { background: #FAFCFF; }
    tr:nth-child(even) { background: #FAFCFF; }
    .stage-cards { display: flex; gap: 1rem; flex-wrap: wrap; margin-bottom: 1.5rem; }
    .stage-card { background: #fff; border: 1px solid rgba(16,30,60,0.12); border-radius: 4px; padding: 0.75rem 1rem; min-width: 120px; }
    .stage-card .count { font-size: 2rem; font-weight: bold; }
    .stage-card-alert { background: #FFF3E0; border-color: #E8870E; }
    .deadline-soon { background: #FCE8E8; color: #DC2626; font-weight: bold; }
    .rec-bid { color: #15A06B; font-weight: bold; }
    .rec-nobid { color: #C2354A; font-weight: bold; }
    .filter-bar { margin-bottom: 0.75rem; font-size: 0.9rem; color: #5A6B86; }
    a { color: #2563EB; }
  </style>
</head>
<body>
  {{.HeaderLockup}}  <!-- dashboard.HeaderLockup(): Kai wave mark + "Kaimi" wordmark + "THE SEEKER"; replaces the plain <h1> -->
  {{template "content" .}}
</body>
</html>
```

The `<meta http-equiv="refresh" content="30">` tag is omitted on the 404 error page since there is nothing new to fetch.

**Design system (canonical source):** the full Kaimi design system — tokens (`kaimi/tokens.css` from the handoff: status vocabulary, fit bands, urgency escalation, type/spacing/radii/elevation/motion, light Triage + dark Focus themes) and component classes (`kaimi/ui.css`: `kbadge`, `krec`, `kdead`, `kfit`, `kbtn`, `kchip`, `ktag`) — is defined once in `internal/dashboard` (`StyleTag()`, issue #132). Layouts emit `StyleTag()` in `<head>` and keep only page-specific rules in their own inline `<style>`. For status, recommendation, deadline, fit-score, and meta-tag display, handlers use the reusable renderers (`StatusBadge`, `RecommendationPill`, `DeadlinePill`/`UrgencyFor`, `FitRing`/`FitBandFor`, `MetaTag`) instead of hand-rolled markup, so every view stays on one vocabulary.

**Brand color mapping** (semantic — color always means the same thing):
- Ink/text: navy `#0A1B3D`; secondary text `#5A6B86`; links and "agent working" blue `#2563EB`
- "A human is needed" amber `#E8870E` on `#FFF3E0` — used ONLY for Awaiting Human Review
- Bid / done green `#15A06B`; No-Bid rose `#C2354A`; failed / critical deadline red `#DC2626`
- Backgrounds are navy-tinted neutrals (`#FBFCFE` page, `#FAFCFF` panels); borders `rgba(16,30,60,0.12)`
- Fonts stay on the system stack (no external fonts, per Technology Constraints); the brand's Figtree/IBM Plex Mono pairing applies to surfaces that may ship external assets in later phases

---

## Template Data Contracts

The handler must pass these structs to the templates. These are the minimum required fields; handlers may embed additional unexported fields.

### OverviewData (passed to `/` template)

```go
type OverviewData struct {
    PageTitle    string
    Cards        []StageCard
    Rows         []TableRow
    ActiveStage  string  // current "stage" filter value, empty if none
    ActiveMinScore int   // current min_score value, 0 if none
    ActiveSort   string  // "deadline" or "score"
    ActiveOrder  string  // "asc" or "desc"
}

type StageCard struct {
    Label string
    Stage string  // matches query-param value
    Count int
    Alert bool    // true for "awaiting_review" card
}

type TableRow struct {
    ID            string
    Title         string
    Agency        string
    NAICSCode     string
    ScoreDisplay  string  // "87.3%" or "—"
    ReasoningSnip string  // first 80 chars + "…" or "—"
    Stage         string  // human-readable label
    DeadlineStr   string  // "2026-06-14" or "—"
    DeadlineSoon  bool    // true if within 7 days
    UpdatedStr    string
}
```

### DetailData (passed to `/opportunity/{id}` template)

```go
type DetailData struct {
    PageTitle        string
    Opp              opportunity.Opportunity
    DerivedStage     string
    ScoreDisplay     string   // "87.3%" or "—"
    DeadlineSoon     bool
    DeadlineStr      string
    ScoredAtStr      string   // "2026-01-02 15:04" or "—"
    SelectedAtStr    string
    PostedDateStr    string
    CreatedAtStr     string
    UpdatedAtStr     string
    RecommendationClass string // "rec-bid", "rec-nobid", or ""
}
```

---

## Non-Goals (explicitly out of scope for Wave 3)

- Pagination (table shows all matching rows; defer to Wave 4 if needed)
- Write operations via the dashboard (no "select" button, no status changes)
- Authentication or session management
- WebSocket live-push (meta-refresh is sufficient for Wave 3)
- External CSS files, icon libraries, or fonts
- Mobile-responsive layout (localhost tool, desktop assumed)
