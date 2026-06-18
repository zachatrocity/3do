import { state } from "../state.js";
import { escapeHTML, sortRecent } from "../utils.js";
import { metricCard, renderMiniQueueItem, renderPrinterPills } from "../components.js";

export function renderDashboard(root) {
  const active = state.queueItems.filter((i) => i.status === "printing");
  const queued = state.queueItems.filter((i) => i.status === "queued");
  const blocked = state.queueItems.filter((i) => i.status === "blocked");
  const backlog = state.queueItems.filter((i) => i.status === "backlog");
  const open = state.queueItems.filter((i) => !["done", "cancelled"].includes(i.status));
  const done = sortRecent(state.queueItems.filter((i) => i.status === "done")).slice(0, 5);
  const columns = [
    ["Printing", active, "No active prints."],
    ["Queued", queued, "No staged jobs."],
    ["Blocked", blocked, "No blockers."],
    ["Backlog", backlog, "No backlog."],
  ];

  root.innerHTML = `
    <section class="metric-grid">
      ${metricCard("Active", active.length, "Jobs under heat", "glow-cyan")}
      ${metricCard("Queued", queued.length, "Ready for plates", "glow-lime")}
      ${metricCard("Blocked", blocked.length, "Needs operator", "glow-danger")}
      ${metricCard("Open", open.length, "Total unfinished", "")}
    </section>

    <section class="dashboard-layout">
      <div class="kanban-board">
        ${columns.map(([title, items, empty]) => kanbanColumn(title, items, empty)).join("")}
      </div>
      <aside class="right-rail">
        <section class="rail-card">
          <div class="card-head">
            <h3>Active printers</h3>
            <a href="#admin-printers">Manage</a>
          </div>
          ${renderPrinterPills(state.printers.slice(0, 8))}
        </section>
        <section class="rail-card">
          <div class="card-head">
            <h3>Recent activity</h3>
            <span>${done.length}</span>
          </div>
          <div class="dashboard-list">
            ${done.length ? done.map(renderMiniQueueItem).join("") : `<p class="muted">No recent completions.</p>`}
          </div>
        </section>
      </aside>
    </section>
  `;

  root.querySelectorAll("[data-item-id]").forEach((button) => {
    button.addEventListener("click", () => {
      window.location.hash = `admin-queue?item=${encodeURIComponent(button.dataset.itemId)}`;
    });
  });
}

function kanbanColumn(title, items, emptyText) {
  return `
    <section class="kanban-column">
      <div class="column-head">
        <h3>${escapeHTML(title)}</h3>
        <span>${items.length}</span>
      </div>
      <div class="dashboard-list">
        ${items.length ? items.slice(0, 7).map(renderMiniQueueItem).join("") : `<p class="muted">${escapeHTML(emptyText)}</p>`}
      </div>
    </section>
  `;
}
