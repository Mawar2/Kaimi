package dashboard

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// This file implements the Submitted archive's "Export BD report" — a CSV with
// headline metrics plus one row per submitted proposal, for planning, board
// decks, and bank conversations (app-submitted.jsx ExportDialog).
//
// The design prototype builds the CSV client-side (Blob + URL.createObjectURL)
// inside a modal with an FY-quarter range picker. The dashboard is server-
// rendered and no-JS, so this is a GET endpoint that streams the CSV directly
// (the "Export report" link on /submitted), honoring the same status filter the
// page is showing. The interactive quarter-range picker is intentionally
// dropped; the report covers the current filtered view.

// fyQuarter returns the US federal fiscal year and quarter for t. The federal
// fiscal year starts Oct 1: Oct–Dec is Q1 of the next FY, Jan–Mar Q2, Apr–Jun
// Q3, Jul–Sep Q4 (matches app-submitted.jsx fyQuarter).
func fyQuarter(t time.Time) (fy, q int) {
	m := int(t.Month()) // 1..12
	fy = t.Year()
	switch {
	case m >= 10:
		fy++
		q = 1
	case m <= 3:
		q = 2
	case m <= 6:
		q = 3
	default:
		q = 4
	}
	return fy, q
}

// fyQuarterLabel renders the short "FY26 Q3" label for a submission date.
func fyQuarterLabel(t time.Time) string {
	fy, q := fyQuarter(t)
	return fmt.Sprintf("FY%02d Q%d", fy%100, q)
}

// handleSubmittedExport streams the BD report CSV for the submitted archive,
// honoring the optional ?status= filter.
func (h *Handler) handleSubmittedExport(w http.ResponseWriter, r *http.Request) {
	now := h.Now()
	rows, err := h.svc.List(r.Context(), ListOptions{Now: now})
	if err != nil {
		http.Error(w, "failed to load the archive", http.StatusInternalServerError)
		return
	}
	filter := r.URL.Query().Get("status")
	switch filter {
	case "pending", "won", "lost":
	default:
		filter = "all"
	}

	type expRow struct {
		title, agency, sol, submitted, fyq, status string
		valueM                                     float64
	}
	var out []expRow
	var totalVal, wonVal float64
	var wonN, lostN int

	for i := range rows {
		if rows[i].Stage != StageSubmitted {
			continue
		}
		opp, gerr := h.svc.Get(r.Context(), rows[i].ID)
		if gerr != nil {
			continue
		}
		cur := opp.AwardOutcome
		if cur == "" {
			cur = "pending"
		}
		if filter != "all" && cur != filter {
			continue
		}
		when := opp.UpdatedAt
		if opp.SubmittedAt != nil {
			when = *opp.SubmittedAt
		}
		valM := opp.EstimatedValue / 1_000_000
		totalVal += valM
		switch opp.AwardOutcome {
		case "won":
			wonVal += valM
			wonN++
		case "lost":
			lostN++
		}
		label, _, _ := awardStatus(opp.AwardOutcome)
		out = append(out, expRow{
			title: opp.Title, agency: opp.Agency, sol: opp.SolicitationNum,
			submitted: when.Format("Jan 2, 2006"), fyq: fyQuarterLabel(when),
			status: label, valueM: valM,
		})
	}

	winRate := "—"
	if decided := wonN + lostN; decided > 0 {
		winRate = strconv.Itoa(wonN*100/decided) + "%"
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="kaimi-bd-report.csv"`)
	cw := csv.NewWriter(w)
	scope := "All submissions"
	if filter != "all" {
		scope = "Filter: " + filter
	}
	_ = cw.Write([]string{"Kaimi BD report", scope})
	_ = cw.Write([]string{"Generated", now.Format("Jan 2, 2006")})
	_ = cw.Write(nil)
	_ = cw.Write([]string{"Proposals submitted", strconv.Itoa(len(out))})
	_ = cw.Write([]string{"Total submitted value ($M)", fmt.Sprintf("%.2f", totalVal)})
	_ = cw.Write([]string{"Won value ($M)", fmt.Sprintf("%.2f", wonVal)})
	_ = cw.Write([]string{"Win rate (decided)", winRate})
	_ = cw.Write(nil)
	_ = cw.Write([]string{"Title", "Agency", "Solicitation", "Submitted", "FY quarter", "Value ($M)", "Status"})
	for i := range out {
		e := &out[i]
		_ = cw.Write([]string{e.title, e.agency, e.sol, e.submitted, e.fyq, fmt.Sprintf("%.2f", e.valueM), e.status})
	}
	cw.Flush()
}
