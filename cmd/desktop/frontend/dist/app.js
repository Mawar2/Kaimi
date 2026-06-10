// Minimal scaffold frontend (issue #138): call the bound Go method and render
// the local store's opportunities. Wails injects window.go.main.App bindings.
// Field names follow the Go struct (no JSON tags on OpportunityRow), e.g. .ID.

function esc(s) {
  return String(s == null ? "" : s).replace(/[&<>"']/g, function (c) {
    return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
  });
}

function fmtScore(v) {
  return v > 0 ? Math.round(v * 100) + "%" : "—";
}

function fmtDeadline(iso) {
  if (!iso) return "—";
  const d = new Date(iso);
  if (isNaN(d) || d.getFullYear() < 2) return "—";
  return d.toISOString().slice(0, 10);
}

function render(result) {
  const app = document.getElementById("app");
  const path = result.storePath ? `<p class="storepath">Local store: ${esc(result.storePath)}</p>` : "";

  if (result.empty || !result.rows || result.rows.length === 0) {
    app.innerHTML = path + `<div class="empty">${esc(result.message || "No opportunities yet.")}</div>`;
    return;
  }

  const head =
    "<tr><th>ID</th><th>Title</th><th>Agency</th><th>NAICS</th><th>Score</th><th>Stage</th><th>Deadline</th></tr>";
  const body = result.rows
    .map(function (r) {
      const deadlineCls = r.DeadlineSoon ? ' class="deadline-soon"' : "";
      return (
        "<tr>" +
        `<td class="mono">${esc(r.ID)}</td>` +
        `<td>${esc(r.Title)}</td>` +
        `<td>${esc(r.Agency)}</td>` +
        `<td class="mono">${esc(r.NAICSCode)}</td>` +
        `<td>${esc(fmtScore(r.Score))}</td>` +
        `<td>${esc(r.Stage)}</td>` +
        `<td${deadlineCls}>${esc(fmtDeadline(r.ResponseDeadline))}</td>` +
        "</tr>"
      );
    })
    .join("");
  app.innerHTML = path + "<table>" + head + body + "</table>";
}

function renderError(err) {
  document.getElementById("app").innerHTML =
    `<div class="error">Could not read the local store: ${esc(err)}</div>`;
}

function load() {
  if (!window.go || !window.go.main || !window.go.main.App) {
    // Bindings not ready yet (early call); retry shortly.
    return setTimeout(load, 50);
  }
  window.go.main.App.ListOpportunities().then(render).catch(renderError);
}

window.addEventListener("DOMContentLoaded", load);
