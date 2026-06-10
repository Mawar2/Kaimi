# Dashboard Design-Consistency Log

**Last updated:** 2026-06-10 · **Owner:** Malik (malik@bluemetatech.com)

Running checklist for driving the Kaimi web dashboard (`internal/dashboard`) to full
visual consistency with the locked design handoff. One surface at a time, audited and
verified **in a real browser** (gstack-browse) against
`design-handoff/Kaimi-handoff/kaimi/project/design_handoff_kaimi/screenshots/`.

Hard rules (from `hackathon/design-consistency-agent.txt`): define design values once and
reuse (`StyleTag()` tokens + `components.go` helpers — never re-hardcode per page); no
external assets/fonts (inline SVG + base64 OK); amber `#E8870E`/`#FFF3E0` is reserved
app-wide for "a human is needed"; any layout change updates `docs/dashboard/ux-spec.md` in
the same change; ticketed + TDD + `make all` green + one PR per surface + a human merges.

## Verification harness (how to reproduce)

```
# in the isolated worktree C:\Users\Owner\Kaimi-design
go run ./cmd/pipeline --mode=cached --store-path=./design-store   # seed render data
go build -o bin/dashboard.exe ./cmd/dashboard
bin\dashboard.exe --store=.\design-store --port=8901             # NB: kill stale PID first
# browser: gstack-browse goto http://127.0.0.1:8901/ → screenshot → diff vs screenshots/
```

Gotcha logged: Windows locks a running `.exe`, so `go build` cannot overwrite a live
`dashboard.exe`. Always stop every listener on the port first (a zombie process will keep
serving the **old** bytes and silently mask your change). Confirm with
`curl -s http://127.0.0.1:8901/ | grep -c @font-face` and a byte-count delta.

## Surface status

| Surface | Route | Reference shot | Audited | Fixed | Browser-verified | 2× clean design-review |
|---|---|---|---|---|---|---|
| Triage | `/` | `01-opportunities.png` | ✅ 06-10 | typography only | ✅ fonts | ☐ |
| Opportunity detail | `/opportunity/{id}` | `02-opportunity-drawer*.png` | ☐ | ☐ | ☐ | ☐ |
| Proposals command | `/proposals` | `03-proposals-command.png` | ☐ | ☐ | ☐ | ☐ |
| Workspace | `/workspace/{id}` | `04/06/07-workspace*.png` | ✅ 06-10 | surfaces → `--surface` | ✅ gate+editor | ☐ |
| Shared chrome (header/nav/states/responsive) | all | — | partial | typography | ✅ fonts | ☐ |
| Component pass (all states) | — | `08-design-system*.png` | ☐ | ☐ | ☐ | ☐ |

## Iteration log

### 2026-06-10 — Global typography: self-host Figtree + Geist Mono

**Divergence (the headline one).** `StyleTag()` declared `--font-sans: "Figtree"` /
`--font-mono: "Geist Mono"` but embedded **no `@font-face`**, so the served UI fell back
to system fonts (Segoe UI on Windows) and drifted from the comps. Confirmed in a real
browser: `document.fonts.size === 0`, no `@font-face` in served HTML, fonts not in the
machine's font dirs. (Note: `document.fonts.check('…Figtree')` returns a misleading `true`
when no matching `@font-face` exists — it means "nothing pending," not "installed." Do not
trust it; check `document.fonts` membership and computed render instead.)

**Decision.** Self-host both faces as inline base64 `@font-face` data-URIs (self-hosting,
not an external fetch — honors the no-external-assets rule). Mono face = **Geist Mono**,
per the design-system token order and Malik's "match the design system as a requirement"
call; the handoff screenshots show IBM Plex Mono only because the comps loaded that
fallback. Variable builds chosen because the type tokens use non-standard weights
(420/430/550/650). SIL OFL, license files shipped.

**Changes.** `internal/dashboard/fonts.go` (new — `//go:embed` + base64 `@font-face`),
`internal/dashboard/fonts/{figtree-variable.woff2,geist-mono-variable.woff2,*-OFL.txt}`,
`StyleTag()` prepends `fontFaceCSS`, stale "falls back to system fonts" comment corrected,
`tokens_test.go` extended (`TestStyleTagSelfHostsDesignedFonts`), `ux-spec.md` updated.

**Verified (real browser, screenshot-diff verdict: PASS).** `document.fonts` →
`Figtree:loaded | Geist Mono:loaded`; H1 renders Figtree, NAICS mono renders Geist Mono;
**0** external font requests; no console errors; served page `+62.5KB` (the embedded
faces). `make all`-equivalent green (module build + all package tests + `golangci-lint`
clean on `internal/dashboard`). Typography now matches the comps.

### 2026-06-10 — Workspace: route editor surfaces through `--surface` (PR pending, issue #210)

**Divergence.** `/workspace/{id}` re-hardcoded `background: #fff` on `.edsec textarea`,
`.draft-body`, and the ready-card gradient (`#fff 60%`) — a hardcode **and** a dark-theme bug
(white surfaces ignore the Focus theme's `--surface`). **Fix:** all three → `var(--surface)`
(theme-aware). Verified at the gate + done states in a real browser; no light-mode regression.

**Tried and reverted (logged for a proper follow-up).** Also tried deduping the gate handoff
avatar (`:226`) through the `agents` map via `{{.Agent.HueBG}}`. This **broke** the avatar:
`html/template`'s style-attribute CSS sanitizer rejects a dynamic `linear-gradient(...)` value
and emits `ZgotmplZ` (the static literal was fine precisely because it isn't interpolated). The
**same latent bug already exists** at the progress-state avatar (`:290`, `style="background:
{{.Agent.HueBG}}"`). Proper fix = type `agentIdentity.HueBG/HueFG` as `template.CSS` (safe — the
values are static map constants, not user input); that fixes `:290` too. Reverted `:226` to the
working literal for this PR; filed as backlog below.

TDD `TestWorkspaceSurfacesUseDesignTokens`; `make all`-green; `golangci-lint` clean.

## Audit backlog (found while auditing; not yet ticketed/fixed)

- **Agent gradient ZgotmplZ:** the progress-state avatar (`proposals_templates.go:290`) renders
  a dynamic `{{.Agent.HueBG}}` gradient in a `style` attr → `html/template` sanitizes it to
  `ZgotmplZ` (broken background). Fix by typing `agentIdentity.HueBG/HueFG` as `template.CSS`;
  this also lets the gate avatar (`:226`) dedup through the `agents` map. Security note: the
  values are static constants from the map, so `template.CSS` is safe here.
- **Workspace success gradient** (`:300,314`, `#2BD49A,#15A06B`) mixes the light + Focus
  `--st-done` greens — no single token; mirrors the handoff. Left as-is.
- **Shared chrome:** `sidebarMarkSVG` (inline brand SVG in `handler.go:100`) duplicates the
  brand mark; consider sourcing it from `brand.go` (`HeaderLockup`) in the shared-chrome pass.
- **Component coverage:** the cached seed only spans BID rows; hand-add opportunity JSONs
  under `./design-store/queue/` covering every RecommendationPill (BID/NO_BID/REVIEW),
  DeadlinePill urgency band, FitRing fit band, and StatusBadge ProposalStatus for the
  dedicated component pass.
