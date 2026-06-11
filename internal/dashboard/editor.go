package dashboard

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/Mawar2/Kaimi/internal/document"
	"github.com/Mawar2/Kaimi/internal/finalreview"
)

// This file implements the full-page draft editor (the new Kaimi App.html
// "editor" route, desktop-editor.jsx + editor.css): a focused, full-bleed
// writing surface — a section rail on the left and the working draft on the
// right, with inline gap callouts. It is a standalone page (no app shell) and
// reuses the proposal Document plus the workspace's section autosave endpoint,
// so editing the draft here is identical to editing it at the review gate.

// EditorData is the view-model for GET /editor/{id}.
type EditorData struct {
	Title        string // page title
	OppID        string
	OppTitle     string
	Meta         string // "DHS · CISA · SOL# … · click any paragraph to edit"
	VersionLabel string
	Sections     []EditorSection
}

// EditorSection is one editable section plus any gap flag attached to it.
type EditorSection struct {
	ID      string
	Heading string
	Status  string
	Body    string
	Flag    *document.Flag // non-nil when the section carries an open review flag
	Gaps    []string       // missing-fact text of each Writer [GAP: ...] marker in Body
}

// gapFlagTitle is the Title the proposal service puts on section-anchored
// unresolved-gap flags (see proposal.flagsFromResult). The editor identifies
// gap flags by it so their callouts can derive from the live body instead.
const gapFlagTitle = "Unresolved gap"

// highlightGaps renders a section body for the read-only draft view with every
// Writer [GAP: ...] marker wrapped in <mark class="gap-mark">. The body is
// HTML-escaped first; escaping never alters the marker's "[GAP:" / "]"
// delimiters, so the boundaries survive.
func highlightGaps(body string) template.HTML {
	escaped := template.HTMLEscapeString(body)
	var b strings.Builder
	for {
		before, after, found := strings.Cut(escaped, "[GAP:")
		if !found {
			b.WriteString(escaped)
			break
		}
		gapText, rest, closed := strings.Cut(after, "]")
		if !closed {
			gapText, rest = after, ""
		}
		b.WriteString(before)
		b.WriteString(`<mark class="gap-mark">[GAP:`)
		b.WriteString(gapText)
		if closed {
			b.WriteString("]")
		}
		b.WriteString(`</mark>`)
		escaped = rest
	}
	return template.HTML(b.String()) //nolint:gosec // input is escaped above; only our own <mark> wrapper is added
}

// gapSummaryData aggregates every unresolved Writer gap across a document's
// sections for the review gate's top-of-page summary (issue #274) — one
// GOV.UK-style block with anchor links instead of a banner per gap.
type gapSummaryData struct {
	Total    int                 // gaps across the whole document
	Headline string              // e.g. "3 unresolved gaps across 2 sections"
	Sections []gapSummarySection // only sections that hold gaps, in order
}

// gapSummarySection is one gap-holding section in the review-gate summary.
type gapSummarySection struct {
	ID      string
	Heading string
	Count   int
}

// summarizeGaps builds the review-gate gap summary from the live section
// bodies, so it agrees with the per-section gap bars by construction.
func summarizeGaps(sections []document.Section) gapSummaryData {
	var d gapSummaryData
	for _, s := range sections {
		n := len(finalreview.GapTexts(s.Body))
		if n == 0 {
			continue
		}
		d.Total += n
		d.Sections = append(d.Sections, gapSummarySection{ID: s.ID, Heading: s.Heading, Count: n})
	}
	d.Headline = fmt.Sprintf("%d unresolved %s across %d %s",
		d.Total, pluralize(d.Total, "gap"), len(d.Sections), pluralize(len(d.Sections), "section"))
	return d
}

// pluralize returns word with an "s" appended unless n is exactly 1.
func pluralize(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// openNonGapFlags returns the document flags the review gate should banner:
// unresolved flags that are NOT per-gap "Unresolved gap" flags. Gaps are
// surfaced by the aggregated summary and section bars instead (issue #274) —
// bannering them too would re-introduce the one-callout-per-gap spam.
func openNonGapFlags(flags []document.Flag) []document.Flag {
	var open []document.Flag
	for _, f := range flags {
		if f.Resolved || f.Title == gapFlagTitle {
			continue
		}
		open = append(open, f)
	}
	return open
}

// handleEditor renders the full-page draft editor for a selected proposal.
func (h *Handler) handleEditor(w http.ResponseWriter, r *http.Request) {
	if h.proposals == nil {
		http.Error(w, "the editor is not enabled on this server", http.StatusServiceUnavailable)
		return
	}
	id := r.PathValue("id")
	if !opportunityIDPattern.MatchString(id) {
		h.renderNotFound(w, id)
		return
	}
	opp, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.renderNotFound(w, id)
		return
	}
	doc, err := h.proposals.Document(id)
	if err != nil || doc == nil {
		h.renderNotFound(w, id)
		return
	}

	// Index open flags by the section they belong to. Unresolved-gap flags are
	// skipped: the gap callouts derive from the live section body instead, so
	// they appear before Vera has run and disappear the moment the human fills
	// the gap — a persisted flag would lag both ways.
	flagBySection := map[string]*document.Flag{}
	for i := range doc.Flags {
		f := &doc.Flags[i]
		if f.Resolved || f.SectionID == "" || f.Title == gapFlagTitle {
			continue
		}
		if _, seen := flagBySection[f.SectionID]; !seen {
			flagBySection[f.SectionID] = f
		}
	}

	data := EditorData{
		Title:        "Editor — " + opp.Title,
		OppID:        id,
		OppTitle:     opp.Title,
		VersionLabel: versionLabel(doc),
	}
	meta := opp.Agency
	if opp.SolicitationNum != "" {
		meta += " · SOL# " + opp.SolicitationNum
	}
	data.Meta = meta + " · click any section to edit"
	for i := range doc.Sections {
		s := &doc.Sections[i]
		data.Sections = append(data.Sections, EditorSection{
			ID: s.ID, Heading: s.Heading, Status: s.Status, Body: s.Body,
			Flag: flagBySection[s.ID],
			Gaps: finalreview.GapTexts(s.Body),
		})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.editorTmpl.Execute(w, data); err != nil {
		fmt.Printf("editor template failed: %v\n", err)
	}
}

// editorPageTmpl is the standalone full-page editor — it deliberately does NOT
// use the app shell (no sidebar); the design's "editor" route is a focused
// full-bleed surface. Section edits autosave through the shared workspace
// endpoint, so the draft stays identical to the review-gate view.
const editorPageTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Kaimi — {{.Title}}</title>
  {{faviconLink}}
  {{styleTag}}
</head>
<body>
<div class="ed-fullpage route-fade">
  <div class="ed">
    <div class="ed-rail">
      <div class="er-h">Sections</div>
      {{range .Sections}}
      <a class="ed-sec{{if or .Flag .Gaps}} warn{{end}}" href="#sec-{{.ID}}"><span class="dot"></span><b>{{.Heading}}</b></a>
      {{end}}
    </div>
    <div class="ed-main">
      <div class="ed-top">
        <a class="kbtn kbtn--ghost kbtn--sm" href="/workspace/{{.OppID}}" style="text-decoration:none">` + iconBack + `Back to review</a>
        <div class="ed-name">Working draft<span>{{.OppTitle}}</span></div>
        <span class="ed-ver you">{{.VersionLabel}}</span>
        <span id="edsave" class="ed-save">Saved</span>
      </div>
      <div class="ed-scroll">
        <div class="ed-doc">
          <div class="ed-title">{{.OppTitle}} — Technical Volume</div>
          <div class="ed-meta">{{.Meta}}</div>
          {{range .Sections}}
          <section id="sec-{{.ID}}" class="edsec">
            <div class="sec-head2"><h3>{{.Heading}}</h3>{{if .Status}}<span class="reqtag">{{.Status}}</span>{{end}}</div>
            <form method="POST" action="/workspace/{{$.OppID}}/section/{{.ID}}" data-autosave>
              <textarea name="body" rows="8"{{if .Gaps}} class="gap-warn"{{end}}>{{.Body}}</textarea>
              <noscript><button class="kbtn kbtn--secondary kbtn--sm" style="margin-top:6px">Save section</button></noscript>
            </form>
            <div class="ed-flag ed-gap" data-gapbar{{if not .Gaps}} hidden{{end}}>
              <span class="ef-ic">` + iconWarn + `</span>
              <div>
                <b data-gapcount>{{len .Gaps}} unresolved gap{{if ne (len .Gaps) 1}}s{{end}}</b>
                <ul class="gap-list" data-gaplist hidden>
                  {{range .Gaps}}<li>{{.}}</li>{{end}}
                </ul>
              </div>
              <button type="button" class="kbtn kbtn--ghost kbtn--sm gap-toggle" data-gaptoggle>Show list</button>
              <button type="button" class="kbtn kbtn--ghost kbtn--sm gap-next" data-gapnext>Next gap &rsaquo;</button>
            </div>
            {{if .Flag}}
            <div class="ed-flag">
              <span class="ef-ic">` + iconWarn + `</span>
              <div><b>{{.Flag.Title}}</b><p>{{.Flag.Detail}}</p></div>
            </div>
            {{end}}
          </section>
          {{end}}
        </div>
      </div>
    </div>
  </div>
</div>
<style>
  .ed-rail .ed-sec{ text-decoration:none; color:inherit; }
  .ed-main .ed-top .kbtn{ color:inherit; }
  .edsec textarea{ width:100%; min-height:120px; font:var(--t-body); color:var(--ink); background:var(--surface); border:1px solid var(--border); border-radius:var(--r-md); padding:12px 14px; resize:vertical; box-sizing:border-box; }
  .edsec textarea:focus{ outline:none; box-shadow:0 0 0 3px var(--ring-focus); border-color:var(--blue-300); }
  .sec-head2{ display:flex; align-items:baseline; gap:9px; margin:0 0 7px; }
  .sec-head2 h3{ font:650 15px/1.3 var(--font-sans); }
  .sec-head2 .reqtag{ font:500 11px/1 var(--font-mono); color:var(--ink-4); }
</style>
<script>
  // Debounced autosave — posts each edited section to the shared workspace
  // endpoint so the draft.md mirror stays current (the one JS-enabled surface).
  var chip = document.getElementById("edsave");
  document.querySelectorAll("form[data-autosave]").forEach(function (f) {
    var area = f.querySelector("textarea"); var timer;
    if (!area) return;
    area.addEventListener("input", function () {
      if (chip) { chip.textContent = "Saving…"; chip.classList.add("saving"); }
      clearTimeout(timer);
      timer = setTimeout(function () {
        fetch(f.action, { method:"POST", headers:{"Content-Type":"application/x-www-form-urlencoded"},
          body: new URLSearchParams(new FormData(f)).toString(), redirect:"manual" })
          .then(function (resp) { if (chip) { chip.textContent = (resp.type==="opaqueredirect"||resp.ok) ? "Saved" : "Save failed"; chip.classList.remove("saving"); } })
          .catch(function () { if (chip) { chip.textContent = "Save failed"; chip.classList.remove("saving"); } });
      }, 900);
    });
  });
` + gapScriptJS + `
</script>
</body>
</html>
`

// gapScriptJS is the client half of the aggregated gap UI (issue #274),
// shared by the full-page editor and the workspace review gate. It keeps each
// section's gap bar live while the human types — count, list, textarea tint,
// rail dot, and the gate summary all derive from the textarea value, so they
// clear the moment the last [GAP: ...] marker is filled (no reload) — and
// drives the "Next gap" cycling and the on-demand gap list.
const gapScriptJS = `
  // Mirrors finalreview.GapTexts: every "[GAP:" marker counts as one gap; its
  // text runs to the closing "]" or, if the model left it unclosed, to the
  // end of its line.
  function gapTexts(text) {
    var gaps = [];
    text.split("\n").forEach(function (line) {
      var from = 0;
      for (;;) {
        var s = line.indexOf("[GAP:", from);
        if (s < 0) break;
        var e = line.indexOf("]", s);
        gaps.push((e < 0 ? line.slice(s + 5) : line.slice(s + 5, e)).trim());
        from = e < 0 ? line.length : e + 1;
      }
    });
    return gaps;
  }
  // Keep the review-gate summary (if this page has one) agreeing with the
  // per-section bars: recount every section, update the per-section rows, and
  // hide the whole block when the draft is clean.
  function updateGapSummary() {
    var sum = document.querySelector("[data-gapsummary]");
    if (!sum) return;
    var total = 0, secs = 0;
    document.querySelectorAll("[data-gapbar]").forEach(function (bar) {
      var sec = bar.closest("section");
      var area = sec && sec.querySelector("textarea");
      if (!area) return;
      var n = gapTexts(area.value).length;
      var li = sec.id ? sum.querySelector('li[data-sec="' + sec.id + '"]') : null;
      if (li) {
        li.hidden = n === 0;
        var c = li.querySelector(".gs-n");
        if (c) c.textContent = n;
      }
      if (n > 0) { total += n; secs += 1; }
    });
    sum.hidden = total === 0;
    var h = sum.querySelector("[data-gapheadline]");
    if (h) h.textContent = total + " unresolved " + (total === 1 ? "gap" : "gaps")
      + " across " + secs + " " + (secs === 1 ? "section" : "sections");
  }
  document.querySelectorAll("[data-gapbar]").forEach(function (bar) {
    var sec = bar.closest("section");
    var area = sec && sec.querySelector("textarea");
    if (!area) return;
    var count = bar.querySelector("[data-gapcount]");
    var list = bar.querySelector("[data-gaplist]");
    var toggle = bar.querySelector("[data-gaptoggle]");
    var next = bar.querySelector("[data-gapnext]");
    var rail = sec.id ? document.querySelector('.ed-sec[href="#' + sec.id + '"]') : null;
    // A non-gap review flag keeps the rail dot amber even once gaps are filled.
    var hasOtherFlag = !!sec.querySelector(".ed-flag:not(.ed-gap)");
    area.addEventListener("input", function () {
      var gaps = gapTexts(area.value);
      bar.hidden = gaps.length === 0;
      area.classList.toggle("gap-warn", gaps.length > 0);
      if (rail) rail.classList.toggle("warn", gaps.length > 0 || hasOtherFlag);
      if (count) count.textContent = gaps.length + " unresolved " + (gaps.length === 1 ? "gap" : "gaps");
      if (list) {
        list.textContent = "";
        gaps.forEach(function (g) {
          var li = document.createElement("li");
          li.textContent = g;
          list.appendChild(li);
        });
      }
      updateGapSummary();
    });
    if (toggle && list) toggle.addEventListener("click", function () {
      list.hidden = !list.hidden;
      toggle.textContent = list.hidden ? "Show list" : "Hide list";
    });
    // Cycle the selection through the [GAP: ...] markers, wrapping after the
    // last, so one button replaces a find-button per gap.
    if (next) next.addEventListener("click", function () {
      var v = area.value;
      var s = v.indexOf("[GAP:", area.selectionEnd || 0);
      if (s < 0) s = v.indexOf("[GAP:");
      if (s < 0) return;
      var close = v.indexOf("]", s), nl = v.indexOf("\n", s);
      var end = close >= 0 && (nl < 0 || close < nl) ? close + 1 : (nl < 0 ? v.length : nl);
      area.focus();
      area.setSelectionRange(s, end);
      area.scrollIntoView({ behavior: "smooth", block: "center" });
    });
  });
`
