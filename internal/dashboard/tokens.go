package dashboard

import "html/template"

// This file embeds the Kaimi design tokens and shared component styles from
// the design handoff (Kaimi Design System.html, kaimi/tokens.css, kaimi/ui.css
// — GitHub issue #132). The token files are the authoritative source of all
// visual values; pages must consume StyleTag rather than re-hardcoding colors,
// so the whole UI stays on one vocabulary.
//
// Semantic rule (load-bearing, app-wide): amber (--st-human #E8870E family)
// means "a human is needed" and NOTHING else. Never use it decoratively.
//
// Delivery is inline-only per docs/dashboard/ux-spec.md: the styles are
// emitted into each page's <head>; no external CSS files or fonts are loaded
// (font stacks fall back to system fonts when Figtree/IBM Plex Mono are not
// installed locally).

// designTokensCSS is kaimi/tokens.css from the handoff, verbatim: brand ramps,
// the status vocabulary, fit bands, urgency escalation, semantic surfaces for
// the light Triage theme and the dark Focus theme, typography, spacing, radii,
// elevation, and motion.
const designTokensCSS = `
/* ============================================================
   KAIMI — Design Tokens
   "the seeker" · a BlueMeta product
   Two modes: light TRIAGE (:root) · dark FOCUS ([data-theme="focus"])
   ============================================================ */

:root {
  /* ---- Brand ramp (BlueMeta house blue) ---- */
  --blue-50:  #EEF4FF;
  --blue-100: #DCE6FF;
  --blue-200: #B9CEFF;
  --blue-300: #8DAEFF;
  --blue-400: #5B86F7;
  --blue-500: #2563EB;   /* BlueMeta primary */
  --blue-600: #1D4ED8;
  --blue-700: #1A3FAE;
  --blue-800: #16327F;
  --blue-900: #0A1B3D;   /* deep navy — the house ink */

  /* ---- Kaimi accent (cyan — the seeker's signal) ---- */
  --cyan-50:  #E7FBFF;
  --cyan-200: #A5EEFB;
  --cyan-300: #67E0F4;
  --cyan-400: #22D3EE;   /* accent */
  --cyan-500: #0EA5C4;
  --cyan-600: #0B7E97;

  /* ---- Neutral (cool, navy-tinted greys) ---- */
  --n-0:   #FFFFFF;
  --n-25:  #FAFCFF;
  --n-50:  #F4F7FC;
  --n-100: #ECF1F9;
  --n-200: #DCE4F0;
  --n-300: #C3CFE1;
  --n-400: #94A3BE;
  --n-500: #64748B;
  --n-600: #475569;
  --n-700: #334155;
  --n-800: #1E293B;
  --n-900: #0F1B30;

  /* ============================================================
     STATUS VOCABULARY — the system's most reused colors
     ============================================================ */

  /* Agent / stage states */
  --st-pending:      #64748B;   /* slate — dormant */
  --st-pending-bg:   #EEF1F6;
  --st-progress:     #2563EB;   /* blue — working */
  --st-progress-bg:  #E7EEFF;
  --st-done:         #15A06B;   /* green — complete */
  --st-done-bg:      #E2F6EE;
  --st-human:        #E8870E;   /* AMBER/GOLD — needs you (loudest) */
  --st-human-tint:   #F6A938;
  --st-human-bg:     #FFF3E0;
  --st-human-glow:   rgba(232,135,14,0.45);
  --st-failed:       #DC2626;   /* red — failed */
  --st-failed-bg:    #FCE8E8;

  /* Bid recommendation */
  --rec-bid:     #15A06B;   /* go — green */
  --rec-bid-bg:  #E2F6EE;
  --rec-nobid:   #C2354A;   /* don't — rose-red */
  --rec-nobid-bg:#FBE9EC;
  --rec-review:  #E8870E;   /* human judgment — amber (same family as Needs Human) */
  --rec-review-bg:#FFF3E0;

  /* Fit-score bands */
  --fit-strong: #15A06B;   /* 80-100 */
  --fit-good:   #0EA5C4;   /* 60-79  */
  --fit-fair:   #E8870E;   /* 40-59  */
  --fit-weak:   #C2354A;   /* 0-39   */
  --fit-track:  #E4EAF3;   /* unfilled ring */

  /* Urgency / deadline escalation */
  --urg-calm:    #64748B;   /* >30d */
  --urg-soon:    #2563EB;   /* 14-30d */
  --urg-near:    #E8870E;   /* 7-14d */
  --urg-crit:    #DC2626;   /* <7d / <72h — pulses */
  --urg-crit-bg: #FCE8E8;

  /* ============================================================
     SEMANTIC SURFACES — light TRIAGE
     ============================================================ */
  --bg:          var(--n-50);
  --bg-sunken:   #EDF2F9;
  --surface:     var(--n-0);
  --surface-2:   var(--n-25);
  --surface-3:   var(--n-100);
  --ink:         var(--blue-900);
  --ink-2:       var(--n-600);
  --ink-3:       var(--n-500);
  --ink-4:       var(--n-400);
  --border:      var(--n-200);
  --border-2:    var(--n-100);
  --primary:     var(--blue-500);
  --primary-ink: #FFFFFF;
  --accent:      var(--cyan-500);
  --ring-focus:  rgba(37,99,235,0.35);

  /* ---- Typography ---- */
  --font-sans: "Figtree", ui-sans-serif, system-ui, -apple-system, sans-serif;
  --font-mono: "Geist Mono", "IBM Plex Mono", ui-monospace, "SF Mono", monospace;

  --t-display: 700 48px/1.04 var(--font-sans);
  --t-h1:      700 33px/1.1  var(--font-sans);
  --t-h2:      650 25px/1.18 var(--font-sans);
  --t-h3:      650 19px/1.28 var(--font-sans);
  --t-body-l:  420 17px/1.55 var(--font-sans);
  --t-body:    420 15px/1.55 var(--font-sans);
  --t-small:   430 13px/1.45 var(--font-sans);
  --t-label:   600 11px/1   var(--font-sans);     /* uppercase tracked */

  /* ---- Spacing (4px base) ---- */
  --s-1: 4px;  --s-2: 8px;  --s-3: 12px; --s-4: 16px; --s-5: 20px;
  --s-6: 24px; --s-7: 32px; --s-8: 40px; --s-9: 48px; --s-10: 64px; --s-12: 80px;

  /* ---- Radius ---- */
  --r-xs: 5px; --r-sm: 8px; --r-md: 11px; --r-lg: 16px; --r-xl: 22px; --r-pill: 999px;

  /* ---- Elevation (light) ---- */
  --e-1: 0 1px 2px rgba(15,27,48,0.06), 0 1px 1px rgba(15,27,48,0.04);
  --e-2: 0 2px 4px rgba(15,27,48,0.06), 0 4px 12px rgba(15,27,48,0.07);
  --e-3: 0 6px 16px rgba(15,27,48,0.10), 0 12px 32px rgba(15,27,48,0.10);
  --e-4: 0 18px 48px rgba(10,27,61,0.18), 0 8px 20px rgba(10,27,61,0.10);

  /* ---- Motion ---- */
  --m-fast: 120ms;
  --m-base: 220ms;
  --m-slow: 360ms;
  --ease:        cubic-bezier(0.22, 0.8, 0.28, 1);
  --ease-out:    cubic-bezier(0.16, 1, 0.3, 1);
  --ease-spring: cubic-bezier(0.34, 1.56, 0.64, 1);
}

/* ============================================================
   DARK — FOCUS MODE
   ============================================================ */
[data-theme="focus"] {
  --bg:        #070E22;
  --bg-sunken: #050A1A;
  --surface:   #0E1A36;
  --surface-2: #122146;
  --surface-3: #16284C;
  --ink:       #EAF1FF;
  --ink-2:     #A8B8DA;
  --ink-3:     #7587AE;
  --ink-4:     #5A6B92;
  --border:    rgba(150,180,230,0.14);
  --border-2:  rgba(150,180,230,0.08);
  --primary:   #3B82F6;
  --primary-ink:#FFFFFF;
  --accent:    var(--cyan-400);
  --ring-focus: rgba(34,211,238,0.45);

  /* status backgrounds get dark, glassy fills */
  --st-pending-bg:  rgba(100,116,139,0.16);
  --st-progress:    #5B9BFF;
  --st-progress-bg: rgba(59,130,246,0.18);
  --st-done:        #2BD49A;
  --st-done-bg:     rgba(21,160,107,0.18);
  --st-human:       #FFB24D;
  --st-human-tint:  #FFC56E;
  --st-human-bg:    rgba(232,135,14,0.18);
  --st-human-glow:  rgba(255,178,77,0.55);
  --st-failed:      #FF6B6B;
  --st-failed-bg:   rgba(220,38,38,0.18);

  --rec-bid:#2BD49A; --rec-bid-bg:rgba(21,160,107,0.18);
  --rec-nobid:#FF7A8A; --rec-nobid-bg:rgba(194,53,74,0.18);
  --rec-review:#FFB24D; --rec-review-bg:rgba(232,135,14,0.18);

  --fit-strong:#2BD49A; --fit-good:#3DD6F0; --fit-fair:#FFB24D; --fit-weak:#FF7A8A;
  --fit-track: rgba(150,180,230,0.16);

  --urg-calm:#7587AE; --urg-soon:#5B9BFF; --urg-near:#FFB24D; --urg-crit:#FF6B6B;
  --urg-crit-bg: rgba(220,38,38,0.18);

  --e-1: 0 1px 2px rgba(0,0,0,0.4);
  --e-2: 0 4px 14px rgba(0,0,0,0.45);
  --e-3: 0 10px 30px rgba(0,0,0,0.5);
  --e-4: 0 24px 60px rgba(0,0,0,0.6);
}

/* ---- base reset ---- */
*, *::before, *::after { box-sizing: border-box; }
html { -webkit-text-size-adjust: 100%; }
body {
  margin: 0;
  font: var(--t-body);
  color: var(--ink);
  background: var(--bg);
  -webkit-font-smoothing: antialiased;
  text-rendering: optimizeLegibility;
}
h1,h2,h3,h4,p { margin: 0; }
button { font-family: inherit; }
::selection { background: var(--cyan-200); color: var(--blue-900); }
[data-theme="focus"] ::selection { background: rgba(34,211,238,0.3); color: #EAF1FF; }

.mono { font-family: var(--font-mono); font-feature-settings: "tnum" 1; }
.label {
  font: var(--t-label);
  text-transform: uppercase;
  letter-spacing: 0.09em;
  color: var(--ink-3);
}
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after { animation-duration: .001ms !important; transition-duration: .001ms !important; }
}
`

// componentStylesCSS is kaimi/ui.css from the handoff, verbatim: the shared
// component classes (status badge, recommendation pill, fit ring, deadline
// pill, buttons, avatar, chips, meta tags) built on the tokens above. Works in
// both themes.
const componentStylesCSS = `
/* ============================================================
   KAIMI — Shared UI components (works in both themes)
   Status vocabulary + core components. Single source of truth.
   ============================================================ */

/* ---------- STATUS BADGE (agent / stage state) ---------- */
.kbadge {
  display: inline-flex; align-items: center; gap: 6px;
  height: 24px; padding: 0 10px 0 8px;
  border-radius: var(--r-pill);
  font: 600 12px/1 var(--font-sans);
  letter-spacing: 0.01em; white-space: nowrap;
  border: 1px solid transparent;
}
.kbadge .dot {
  width: 7px; height: 7px; border-radius: 50%; background: currentColor;
  flex: none;
}
.kbadge--pending  { color: var(--st-pending);  background: var(--st-pending-bg);  border-color: color-mix(in oklab, var(--st-pending) 22%, transparent); }
.kbadge--progress { color: var(--st-progress); background: var(--st-progress-bg); border-color: color-mix(in oklab, var(--st-progress) 28%, transparent); }
.kbadge--done     { color: var(--st-done);     background: var(--st-done-bg);     border-color: color-mix(in oklab, var(--st-done) 28%, transparent); }
.kbadge--failed   { color: var(--st-failed);   background: var(--st-failed-bg);   border-color: color-mix(in oklab, var(--st-failed) 28%, transparent); }

/* Needs Human — the loudest. Solid amber, glow halo, gentle pulse. */
.kbadge--human {
  color: #fff;
  background: linear-gradient(180deg, var(--st-human-tint), var(--st-human));
  border-color: color-mix(in oklab, var(--st-human) 50%, transparent);
  box-shadow: 0 0 0 0 var(--st-human-glow);
  animation: kHumanPulse 2.2s var(--ease) infinite;
}
.kbadge--human .dot { background: #fff; box-shadow: 0 0 6px rgba(255,255,255,0.9); }
@keyframes kHumanPulse {
  0%   { box-shadow: 0 0 0 0 var(--st-human-glow); }
  60%  { box-shadow: 0 0 0 7px transparent; }
  100% { box-shadow: 0 0 0 0 transparent; }
}

/* In-progress dot animates */
.kbadge--progress .dot { animation: kBlink 1.3s ease-in-out infinite; }
@keyframes kBlink { 0%,100%{opacity:1} 50%{opacity:.35} }

/* ---------- RECOMMENDATION PILL ---------- */
.krec {
  display: inline-flex; align-items: center; gap: 6px;
  height: 26px; padding: 0 12px;
  border-radius: var(--r-sm);
  font: 700 12px/1 var(--font-sans);
  letter-spacing: 0.06em; text-transform: uppercase;
  border: 1px solid transparent;
}
.krec svg { width: 13px; height: 13px; }
.krec--bid    { color: var(--rec-bid);    background: var(--rec-bid-bg);    border-color: color-mix(in oklab, var(--rec-bid) 32%, transparent); }
.krec--nobid  { color: var(--rec-nobid);  background: var(--rec-nobid-bg);  border-color: color-mix(in oklab, var(--rec-nobid) 32%, transparent); }
.krec--review { color: var(--rec-review); background: var(--rec-review-bg); border-color: color-mix(in oklab, var(--rec-review) 36%, transparent); }

/* ---------- FIT-SCORE RING ---------- */
.kfit { position: relative; display: inline-grid; place-items: center; flex: none; }
.kfit svg { transform: rotate(-90deg); display: block; }
.kfit .kfit-track { stroke: var(--fit-track); }
.kfit .kfit-val { stroke-linecap: round; transition: stroke-dashoffset var(--m-slow) var(--ease); }
.kfit-num {
  position: absolute; display: grid; place-items: center; inset: 0;
  font-family: var(--font-mono); font-weight: 600; font-feature-settings: "tnum" 1;
  line-height: 1; color: var(--ink);
}
.kfit-num small { font-size: 0.5em; color: var(--ink-3); font-weight: 500; margin-top: 1px; }
.kfit[data-band="strong"] .kfit-val { stroke: var(--fit-strong); }
.kfit[data-band="good"]   .kfit-val { stroke: var(--fit-good); }
.kfit[data-band="fair"]   .kfit-val { stroke: var(--fit-fair); }
.kfit[data-band="weak"]   .kfit-val { stroke: var(--fit-weak); }

/* ---------- DEADLINE / URGENCY PILL ---------- */
.kdead {
  display: inline-flex; align-items: center; gap: 6px;
  height: 24px; padding: 0 10px;
  border-radius: var(--r-pill);
  font: 600 12px/1 var(--font-sans); white-space: nowrap;
  color: var(--urg-calm);
  background: color-mix(in oklab, var(--urg-calm) 12%, transparent);
}
.kdead svg { width: 13px; height: 13px; }
.kdead--soon { color: var(--urg-soon); background: color-mix(in oklab, var(--urg-soon) 12%, transparent); }
.kdead--near { color: var(--urg-near); background: color-mix(in oklab, var(--urg-near) 14%, transparent); }
.kdead--crit {
  color: #fff;
  background: linear-gradient(180deg, color-mix(in oklab, var(--urg-crit) 86%, white), var(--urg-crit));
  animation: kCritPulse 1.6s var(--ease) infinite;
}
@keyframes kCritPulse {
  0%,100% { box-shadow: 0 0 0 0 color-mix(in oklab, var(--urg-crit) 50%, transparent); }
  50%     { box-shadow: 0 0 0 5px transparent; }
}

/* ---------- BUTTONS ---------- */
.kbtn {
  --bh: 40px;
  display: inline-flex; align-items: center; justify-content: center; gap: 8px;
  height: var(--bh); padding: 0 18px;
  border-radius: var(--r-md); border: 1px solid transparent;
  font: 600 14px/1 var(--font-sans); cursor: pointer;
  transition: transform var(--m-fast) var(--ease), background var(--m-fast), box-shadow var(--m-fast), border-color var(--m-fast);
  white-space: nowrap; user-select: none;
}
.kbtn svg { width: 16px; height: 16px; }
.kbtn:active { transform: translateY(1px) scale(0.99); }
.kbtn:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--ring-focus); }

.kbtn--ghost     { background: transparent; color: var(--ink-2); border-color: var(--border); }
.kbtn--ghost:hover { background: var(--surface-3); color: var(--ink); }
.kbtn--secondary { background: var(--surface); color: var(--ink); border-color: var(--border); box-shadow: var(--e-1); }
.kbtn--secondary:hover { border-color: var(--n-300); box-shadow: var(--e-2); }
.kbtn--primary   { background: var(--primary); color: var(--primary-ink); box-shadow: var(--e-2), inset 0 1px 0 rgba(255,255,255,0.18); }
.kbtn--primary:hover { background: color-mix(in oklab, var(--primary) 88%, black); box-shadow: var(--e-3); }

/* Select — the threshold-crossing action (cyan, confident) */
.kbtn--select {
  background: linear-gradient(180deg, var(--cyan-400), var(--cyan-500));
  color: #042530; font-weight: 700;
  box-shadow: 0 6px 18px rgba(14,165,196,0.4), inset 0 1px 0 rgba(255,255,255,0.5);
}
.kbtn--select:hover { box-shadow: 0 10px 26px rgba(14,165,196,0.5); transform: translateY(-1px); }

/* Approve — the gate's weighty green action */
.kbtn--approve {
  background: linear-gradient(180deg, color-mix(in oklab, var(--st-done) 88%, white), var(--st-done));
  color: #fff; font-weight: 700;
  box-shadow: 0 6px 18px color-mix(in oklab, var(--st-done) 45%, transparent), inset 0 1px 0 rgba(255,255,255,0.35);
}
.kbtn--approve:hover { transform: translateY(-1px); box-shadow: 0 10px 26px color-mix(in oklab, var(--st-done) 55%, transparent); }

/* Request changes — its weighty counterpart */
.kbtn--changes { background: var(--surface); color: var(--st-human); border-color: color-mix(in oklab, var(--st-human) 45%, transparent); }
.kbtn--changes:hover { background: var(--st-human-bg); }

.kbtn--lg { --bh: 52px; padding: 0 26px; font-size: 16px; border-radius: var(--r-lg); }
.kbtn--sm { --bh: 32px; padding: 0 12px; font-size: 13px; border-radius: var(--r-sm); }
.kbtn:disabled { opacity: 0.5; cursor: not-allowed; }

/* ---------- AVATAR (agent teammate) ---------- */
.kava {
  display: inline-grid; place-items: center; flex: none;
  width: 36px; height: 36px; border-radius: 11px;
  font: 700 13px/1 var(--font-sans); color: #fff;
  background: var(--blue-500); position: relative;
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.2);
}
.kava--sm { width: 28px; height: 28px; border-radius: 9px; font-size: 11px; }
.kava--lg { width: 48px; height: 48px; border-radius: 14px; font-size: 17px; }

/* ---------- CHIP (filter / sort / meta) ---------- */
.kchip {
  display: inline-flex; align-items: center; gap: 6px;
  height: 30px; padding: 0 12px; border-radius: var(--r-pill);
  font: 500 13px/1 var(--font-sans); color: var(--ink-2);
  background: var(--surface); border: 1px solid var(--border);
  cursor: pointer; transition: all var(--m-fast) var(--ease);
}
.kchip:hover { border-color: var(--n-300); color: var(--ink); }
.kchip--on { background: var(--blue-50); color: var(--blue-600); border-color: color-mix(in oklab, var(--primary) 35%, transparent); }
[data-theme="focus"] .kchip--on { background: var(--st-progress-bg); color: #9CC2FF; }
.kchip svg { width: 14px; height: 14px; }

/* meta-row tag (NAICS etc) */
.ktag {
  display:inline-flex; align-items:center; gap:5px;
  font: 500 12px/1 var(--font-mono); color: var(--ink-3);
  padding: 3px 7px; border-radius: var(--r-xs);
  background: var(--surface-3); border: 1px solid var(--border-2);
}
`

// StyleTag renders the complete Kaimi design system — tokens plus component
// classes — as a single inline <style> element for a page's <head>. This is
// the one place visual values are defined; see the file comment for the rules.
func StyleTag() template.HTML {
	// #nosec G203 -- constant stylesheets, no user input.
	return template.HTML("<style>" + designTokensCSS + componentStylesCSS + "</style>")
}
