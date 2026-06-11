/* ============================================================
   KAIMI Desktop — bridge to the Go backend.
   Calls the Wails-bound App.ListOpportunities() and maps the
   dashboard rows into the design's opportunity shape. Falls back
   to the bundled demo queue when no local store data is present
   (or when running in a plain browser without Wails bindings).
   ============================================================ */
import { KAIMI_OPPS, KAIMI_PROPOSALS } from './data.js';

// app returns the Wails-bound backend object, or null in a plain browser (vite
// dev / a non-Wails host). Every live call funnels through this so the UI
// degrades to the bundled demo data when the bindings are absent.
function app(){
  return (window.go && window.go.main && window.go.main.App) || null;
}

// Map the backend recommendation ("BID"/"NO_BID"/"REVIEW") to the design's
// rec key. Falls back to a fit-band proxy when the Scorer hasn't set one.
function recFromBackend(rec, fit){
  switch ((rec || "").toUpperCase()) {
    case "BID":    return "bid";
    case "NO_BID": return "nobid";
    case "REVIEW": return "review";
    default:       return fit >= 70 ? "bid" : fit >= 40 ? "review" : "nobid";
  }
}

// deadline label + escalation level from an ISO date string.
function deadlineInfo(iso){
  if(!iso) return { label:"—", level:"calm" };
  const d = new Date(iso);
  if(isNaN(d) || d.getFullYear() < 2) return { label:"—", level:"calm" };
  const days = Math.round((d - new Date()) / 86400000);
  if(days < 0) return { label:"closed", level:"crit" };
  const label = days === 0 ? "today" : days === 1 ? "1 day" : `${days} days`;
  const level = days < 7 ? "crit" : days < 14 ? "near" : days <= 30 ? "soon" : "calm";
  return { label, level };
}

export async function getOpportunities(){
  try {
    const App = window.go && window.go.main && window.go.main.App;
    if(!App || typeof App.ListOpportunities !== "function") return KAIMI_OPPS;
    const res = await App.ListOpportunities();
    if(!res || res.empty || !Array.isArray(res.rows) || res.rows.length === 0) return KAIMI_OPPS;
    return res.rows.map((r, i) => {
      const fit = Math.round((r.Score || 0) * 100);
      const dl = deadlineInfo(r.ResponseDeadline);
      return {
        id: r.ID || ("o" + i),
        title: r.Title || "(untitled opportunity)",
        agency: r.Agency || "",
        naics: r.NAICSCode || "",
        sol: r.SolicitationNum || "",
        fit,
        rec: recFromBackend(r.Recommendation, fit),
        deadlineLabel: dl.label,
        deadlineLevel: r.DeadlineSoon ? "crit" : dl.level,
        isNew: true,
        day: "today",
        value: "",
      };
    });
  } catch (e) {
    return KAIMI_OPPS;
  }
}

// ---- Zone 2 (proposals / workspace / gate) -------------------------------
// Each call uses the live Wails backend when present, and falls back to the
// bundled demo data (or a no-op) in a plain browser so vite dev still renders.

// getProposals returns the active-proposal cards for the command view, mapped
// into the screen's shape. The backend derives card state via internal/zone2view
// — the same source the web uses — so the desktop and web agree (B2).
export async function getProposals(){
  const A = app();
  if(!A || typeof A.ListProposals !== "function") return KAIMI_PROPOSALS;
  try {
    const res = await A.ListProposals();
    if(!res || !Array.isArray(res.cards)) return KAIMI_PROPOSALS;
    return res.cards.map(c => {
      const dl = deadlineInfo(c.deadline);
      return {
        id: c.id, title: c.title, agency: c.agency, when: c.when,
        stageIndex: c.stageIndex, status: c.state, agents: 0,
        deadlineLabel: dl.label, deadlineLevel: dl.level,
      };
    });
  } catch (e) {
    return KAIMI_PROPOSALS;
  }
}

// getWorkspace returns the single-proposal view-model from the backend, or null
// in a plain browser (the caller keeps its mock proposal then).
export async function getWorkspace(id){
  const A = app();
  if(!A || typeof A.Workspace !== "function") return null;
  try {
    return await A.Workspace(id);
  } catch (e) {
    return null;
  }
}

// callAction invokes a bound mutating method, resolving true on success and
// false when the binding is absent (browser/dev) or the call fails — callers
// keep their offline/mock behavior in that case.
async function callAction(name, ...args){
  const A = app();
  if(!A || typeof A[name] !== "function") return false;
  try {
    await A[name](...args);
    return true;
  } catch (e) {
    return false;
  }
}

export const pursue         = (id)            => callAction("Select", id);
export const approveProposal = (id)           => callAction("Approve", id);
export const requestChanges = (id, note)      => callAction("RequestChanges", id, note);
export const submitProposal = (id)            => callAction("Submit", id);
export const updateSection  = (id, sid, body) => callAction("UpdateSection", id, sid, body);

// draftMarkdown returns the proposal's working draft as Markdown for download
// (B3), or "" when unavailable.
export async function draftMarkdown(id){
  const A = app();
  if(!A || typeof A.DraftMarkdown !== "function") return "";
  try {
    return await A.DraftMarkdown(id);
  } catch (e) {
    return "";
  }
}
