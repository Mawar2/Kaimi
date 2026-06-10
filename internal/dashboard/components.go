package dashboard

import (
	"fmt"
	"html/template"
	"math"
)

// This file implements the Kaimi design system's core components (GitHub
// issue #132) as server-side renderers. Each function returns markup built on
// the classes that StyleTag emits, so every screen composes the same parts —
// the design system's "define once, reuse everywhere" rule.
//
// All caller-provided text is HTML-escaped here; callers never pre-escape.

// StatusKind is the agent/stage state vocabulary — the most reused signal in
// the product. Color always means the same thing; StatusHuman (amber) is
// reserved exclusively for "a human is needed".
type StatusKind string

const (
	// StatusPending is dormant: slate, quiet — not yet started.
	StatusPending StatusKind = "pending"
	// StatusProgress means an agent is working: blue, the dot blinks.
	StatusProgress StatusKind = "progress"
	// StatusDone means the stage is complete: green, settled.
	StatusDone StatusKind = "done"
	// StatusHuman means the system needs a person: solid amber, glowing,
	// gently pulsing — the loudest signal in the vocabulary.
	StatusHuman StatusKind = "human"
	// StatusFailed means an agent errored: red, needs attention.
	StatusFailed StatusKind = "failed"
)

// statusKindKnown reports whether kind is part of the vocabulary.
func statusKindKnown(kind StatusKind) bool {
	switch kind {
	case StatusPending, StatusProgress, StatusDone, StatusHuman, StatusFailed:
		return true
	}
	return false
}

// StatusBadge renders the status-vocabulary pill (leading dot + label) for
// kind. Unknown kinds render as pending — the quietest state — so a bad value
// never borrows a loud color.
func StatusBadge(kind StatusKind, label string) template.HTML {
	if !statusKindKnown(kind) {
		kind = StatusPending
	}
	// #nosec G203 -- label is escaped; everything else is constant.
	return template.HTML(fmt.Sprintf(
		`<span class="kbadge kbadge--%s"><span class="dot"></span>%s</span>`,
		kind, template.HTMLEscapeString(label)))
}

// recommendation pill content per scorer value (opportunity.Recommendation).
var recPills = map[string]struct {
	class string
	icon  string
	label string
}{
	"BID": {
		class: "krec--bid",
		icon:  `<svg viewBox="0 0 24 24" fill="none"><path d="M5 13l4 4L19 7" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"/></svg>`,
		label: "Bid",
	},
	"NO_BID": {
		class: "krec--nobid",
		icon:  `<svg viewBox="0 0 24 24" fill="none"><path d="M6 6l12 12M18 6L6 18" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/></svg>`,
		label: "No Bid",
	},
	"REVIEW": {
		class: "krec--review",
		icon:  `<svg viewBox="0 0 24 24" fill="none"><path d="M12 8v5M12 16.5v.01" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/><circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="2"/></svg>`,
		label: "Review",
	},
}

// RecommendationPill renders the bid-recommendation pill for rec ("BID",
// "NO_BID", or "REVIEW", as produced by the scorer). Any other value —
// including the empty string of an unscored opportunity — renders nothing;
// callers show their own placeholder.
func RecommendationPill(rec string) template.HTML {
	p, ok := recPills[rec]
	if !ok {
		return ""
	}
	// #nosec G203 -- constant markup selected by key, no user input.
	return template.HTML(fmt.Sprintf(`<span class="krec %s">%s%s</span>`, p.class, p.icon, p.label))
}

// UrgencyLevel is the deadline-escalation vocabulary: the pill escalates as
// the close date approaches.
type UrgencyLevel string

const (
	// UrgencyCalm is more than 30 days out. Just a date.
	UrgencyCalm UrgencyLevel = "calm"
	// UrgencySoon is 15-30 days out. Blue, counting down.
	UrgencySoon UrgencyLevel = "soon"
	// UrgencyNear is 7-14 days out. Amber — lean in.
	UrgencyNear UrgencyLevel = "near"
	// UrgencyCrit is under 7 days out (or already past). Red, bold, pulsing.
	UrgencyCrit UrgencyLevel = "crit"
)

// UrgencyFor maps days-until-deadline to the escalation level.
func UrgencyFor(daysLeft int) UrgencyLevel {
	switch {
	case daysLeft < 7:
		return UrgencyCrit
	case daysLeft <= 14:
		return UrgencyNear
	case daysLeft <= 30:
		return UrgencySoon
	default:
		return UrgencyCalm
	}
}

// deadlineClockIcon is the shared clock glyph inside every deadline pill.
const deadlineClockIcon = `<svg viewBox="0 0 24 24" fill="none"><circle cx="12" cy="13" r="8" stroke="currentColor" stroke-width="2"/><path d="M12 9v4l3 2" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>`

// DeadlinePill renders the deadline pill with the urgency styling derived
// from daysLeft. label is display text such as "Apr 30" or "Closes in 2d".
func DeadlinePill(label string, daysLeft int) template.HTML {
	class := "kdead"
	if level := UrgencyFor(daysLeft); level != UrgencyCalm {
		class = fmt.Sprintf("kdead kdead--%s", level)
	}
	// #nosec G203 -- label is escaped; everything else is constant.
	return template.HTML(fmt.Sprintf(`<span class=%q>%s%s</span>`,
		class, deadlineClockIcon, template.HTMLEscapeString(label)))
}

// FitBand is the fit-score color band.
type FitBand string

const (
	// FitStrong is 80-100.
	FitStrong FitBand = "strong"
	// FitGood is 60-79.
	FitGood FitBand = "good"
	// FitFair is 40-59.
	FitFair FitBand = "fair"
	// FitWeak is 0-39.
	FitWeak FitBand = "weak"
)

// FitBandFor maps a 0-100 fit score to its color band.
func FitBandFor(score int) FitBand {
	switch {
	case score >= 80:
		return FitStrong
	case score >= 60:
		return FitGood
	case score >= 40:
		return FitFair
	default:
		return FitWeak
	}
}

// fitRingSublabelMinPx is the size at and above which the ring carries the
// small "FIT" sublabel under the number (per the handoff README).
const fitRingSublabelMinPx = 92

// FitRing renders the fit-score ring: an SVG ring that fills clockwise from
// 12 o'clock to score percent, colored by band, with the monospace score
// centered. score is 0-100 (clamped); sizePx is the rendered diameter.
//
// Geometry follows the design system: stroke is ~11% of the diameter and the
// ring rotation comes from the .kfit CSS. The centered number is sized at
// ~34% of the diameter (~29% once the FIT sublabel appears at >=92px),
// matching the handoff specimens.
func FitRing(score, sizePx int) template.HTML {
	score = min(100, max(0, score))

	stroke := int(math.Round(0.11 * float64(sizePx)))
	radius := (sizePx - stroke) / 2
	circumference := 2 * math.Pi * float64(radius)
	// The unfilled remainder of the ring.
	offset := circumference * float64(100-score) / 100

	numberPx := int(math.Round(0.34 * float64(sizePx)))
	sublabel := ""
	if sizePx >= fitRingSublabelMinPx {
		numberPx = int(math.Round(0.29 * float64(sizePx)))
		sublabel = "<small>FIT</small>"
	}

	// #nosec G203 -- numeric values and constant markup only.
	return template.HTML(fmt.Sprintf(
		`<div class="kfit" data-band="%s" style="width:%dpx;height:%dpx">`+
			`<svg width="%d" height="%d" viewBox="0 0 %d %d">`+
			`<circle class="kfit-track" cx="%d" cy="%d" r="%d" fill="none" stroke-width="%d"/>`+
			`<circle class="kfit-val" cx="%d" cy="%d" r="%d" fill="none" stroke-width="%d" stroke-dasharray="%.1f" stroke-dashoffset="%.1f"/>`+
			`</svg>`+
			`<div class="kfit-num" style="font-size:%dpx">%d%s</div>`+
			`</div>`,
		FitBandFor(score), sizePx, sizePx,
		sizePx, sizePx, sizePx, sizePx,
		sizePx/2, sizePx/2, radius, stroke,
		sizePx/2, sizePx/2, radius, stroke, circumference, offset,
		numberPx, score, sublabel))
}

// MetaTag renders the small monospace meta chip used for technical
// identifiers like "NAICS 541512" or "SOL# 70RCSA24R0123".
func MetaTag(text string) template.HTML {
	// #nosec G203 -- text is escaped; everything else is constant.
	return template.HTML(fmt.Sprintf(`<span class="ktag">%s</span>`,
		template.HTMLEscapeString(text)))
}
