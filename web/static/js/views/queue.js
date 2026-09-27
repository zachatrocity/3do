import { apiClient } from "../api.js";
import { refreshData } from "../data.js";
import { renderDashboard } from "./dashboard.js";
import { routeTo, renderCurrentRoute } from "../router.js";
import { routeHref, statuses } from "../navigation.js";
import { state } from "../state.js";
import { escapeAttr, escapeHTML, formatDateInput } from "../utils.js";
import { renderFiles, renderLinks, renderNotes, renderQueueCard, renderStatusEvents } from "../components.js";
import { renderItemThumbnail } from "../thumbnails.js";

export function renderQueue(root, params = {}) {
  if (params.item) {
    root.innerHTML = `<section class="detail-panel item-page"><div id="item-detail" class="detail"></div></section>`;
    loadItemDetail(params.item, root.querySelector("#item-detail"));
    return;
  }
  const view = params.view || "board";
  const items = params.status ? state.queueItems.filter(item => item.status === params.status) : state.queueItems;
  root.innerHTML = `
    <section class="queue-toolbar" aria-label="Queue controls">
      <nav class="view-switch" aria-label="Queue view">
        ${["board", "list"].map(mode => `<a href="${escapeAttr(routeHref("queue", { ...params, view: mode }))}" ${view === mode ? 'aria-current="page"' : ""}>${mode === "board" ? "Board" : "List"}</a>`).join("")}
      </nav>
      <select id="status-filter" aria-label="Filter by status">
        <option value="">All statuses</option>
        ${statuses.map(status => `<option value="${status}" ${params.status === status ? "selected" : ""}>${status[0].toUpperCase() + status.slice(1)}</option>`).join("")}
      </select>
      <button id="refresh-queue" class="secondary" type="button">Refresh</button>
      <p id="queue-status" class="form-status" role="status"></p>
    </section>
    <div id="queue-content"></div>`;
  const content = root.querySelector("#queue-content");
  if (view === "board") renderDashboard(content, params);
  else content.innerHTML = `<div class="queue-list">${items.length ? items.map(item => renderQueueCard(item, null, routeHref("queue", { ...params, item: item.id }))).join("") : '<p class="muted">No prints match this view.</p>'}</div>`;
  root.querySelector("#status-filter").addEventListener("change", event => routeTo("queue", { ...params, status: event.target.value }));
  root.querySelector("#refresh-queue").addEventListener("click", async event => {
    const button = event.currentTarget;
    button.disabled = true;
    try {
      await refreshData();
      if (button.isConnected) renderCurrentRoute();
    } catch (error) {
      if (button.isConnected) root.querySelector("#queue-status").textContent = error.message;
    } finally { button.disabled = false; }
  });
}

async function loadItemDetail(id, detail, message = "") {
  detail.innerHTML = `<p class="muted">Loading print details...</p>`;
  try {
    const item = await apiClient.queueItem(id);
    if (!detail.isConnected) return;
    detail.innerHTML = renderDetail(item);
    detail.querySelector("#detail-status").textContent = message;
    detail.querySelector("#detail-form").addEventListener("submit", event => saveItemDetail(event, id, detail));
    detail.querySelector("#note-form").addEventListener("submit", event => addItemNote(event, id, detail));
  } catch (error) {
    if (detail.isConnected) detail.innerHTML = `<p role="alert">${escapeHTML(error.message)}</p>`;
  }
}

function renderDetail(item) {
  return `
    ${renderItemThumbnail(item, "large")}
    <div class="detail-header">
      <div>
        <h3>${escapeHTML(item.title)}</h3>
        <p class="muted">${escapeHTML(item.requested_by || "No requester")}</p>
      </div>
      <span class="badge status-${escapeAttr(item.status)}">${escapeHTML(item.status)}</span>
    </div>
    <p>${escapeHTML(item.description || "No notes on the request.")}</p>
    ${renderLinks(item.links || [])}
    ${renderFiles(item.files || [])}
    <form id="detail-form" class="detail-form">
      <div class="form-grid">
        ${selectField("status", "Status", item.status, ["backlog", "queued", "printing", "blocked", "done", "cancelled"])}
        ${selectField("priority", "Priority", item.priority, ["low", "normal", "high", "urgent"])}
        <label>Owner<input name="owner" value="${escapeAttr(item.owner)}"></label>
        <label>Printing by<input name="printing_by" value="${escapeAttr(item.printing_by)}"></label>
        <label>Material<input name="material" value="${escapeAttr(item.material)}"></label>
        <label>Color<input name="color" value="${escapeAttr(item.color)}"></label>
        <label>Quantity<input name="quantity" type="number" min="1" value="${escapeAttr(item.quantity || 1)}"></label>
        <label>Estimate (min)<input name="estimated_minutes" type="number" min="1" value="${escapeAttr(item.estimated_minutes || "")}"></label>
      </div>
      <label>Due date<input name="due_at" type="date" value="${escapeAttr(formatDateInput(item.due_at))}"></label>
      <label>Status note<textarea name="status_note" rows="2" placeholder="Brief reason for the status change"></textarea></label>
      <button type="submit">Save changes</button>
      <p id="detail-status" class="form-status" role="status"></p>
    </form>
    <section class="subsection">
      <h3>Notes</h3>
      <form id="note-form">
        <label>New note<textarea name="body" rows="3" required placeholder="Add a comment..."></textarea></label>
        <button type="submit">Post note</button><p class="form-status" role="status"></p>
      </form>
      <div class="timeline">${renderNotes(item.notes || [])}</div>
    </section>
    <section class="subsection">
      <h3>Status history</h3>
      <div class="timeline">${renderStatusEvents(item.status_events || [])}</div>
    </section>
  `;
}

function selectField(name, label, value, options) {
  return `<label>${label}<select name="${name}">${options.map((option) => (
    `<option value="${option}"${value === option ? " selected" : ""}>${option}</option>`
  )).join("")}</select></label>`;
}

async function saveItemDetail(event, id, detail) {
  event.preventDefault();
  const form = event.currentTarget;
  const statusEl = form.querySelector("#detail-status");
  const button = form.querySelector("button[type=submit]");
  const payload = Object.fromEntries(new FormData(form).entries());
  payload.quantity = Number(payload.quantity || 1);
  button.disabled = true;
  statusEl.textContent = "Saving...";
  try {
    await apiClient.updateQueueItem(id, payload);
    statusEl.textContent = "Saved.";
    await refreshData();
    if (detail.isConnected) await loadItemDetail(id, detail, "Saved.");
  } catch (error) { statusEl.textContent = error.message; }
  finally { button.disabled = false; }
}

async function addItemNote(event, id, detail) {
  event.preventDefault();
  const form = event.currentTarget;
  const button = form.querySelector("button[type=submit]");
  const status = form.querySelector(".form-status");
  button.disabled = true;
  try {
    await apiClient.addQueueItemNote(id, Object.fromEntries(new FormData(form).entries()));
    if (detail.isConnected) await loadItemDetail(id, detail, "Note added.");
  } catch (error) { status.textContent = error.message; }
  finally { button.disabled = false; }
}
