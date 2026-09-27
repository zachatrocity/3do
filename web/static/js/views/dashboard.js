import { state } from "../state.js";
import { escapeHTML, escapeAttr, itemSubtitle } from "../utils.js";
import { statuses, routeHref } from "../navigation.js";

// The board and list are views of the same queue, not separate destinations.
export function renderDashboard(root, params = {}) {
  const visibleStatuses = params.status ? [params.status] : statuses;
  root.innerHTML = `<div class="kanban-board">${visibleStatuses.map(status => {
    const items = state.queueItems.filter(item => item.status === status);
    return `<section class="kanban-column">
      <div class="column-head"><h2>${status[0].toUpperCase() + status.slice(1)}</h2><span>${items.length}</span></div>
      <div class="dashboard-list">${items.length ? items.map(item => `
        <a class="mini-item" href="${escapeAttr(routeHref("queue", { ...params, view: "board", item: item.id }))}">
          <span><strong>${escapeHTML(item.title)}</strong><small>${escapeHTML(itemSubtitle(item))}</small></span>
          ${["high", "urgent"].includes(item.priority) ? `<span class="badge">${escapeHTML(item.priority)}</span>` : ""}
        </a>`).join("") : '<p class="muted">No prints.</p>'}</div>
    </section>`;
  }).join("")}</div>`;
}
