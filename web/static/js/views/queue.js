import { apiClient } from "../api.js";
import { refreshData } from "../data.js";
import { state } from "../state.js";
import { escapeAttr, escapeHTML, formatDateInput } from "../utils.js";
import { renderFiles, renderLinks, renderNotes, renderQueueCard, renderStatusEvents } from "../components.js";
import { renderItemThumbnail } from "../thumbnails.js";

export function renderQueue(root, params = {}) {
  if (params.item) state.selectedItemId = Number(params.item);
  root.innerHTML = `
    <section class="queue-route">
      <div class="queue-toolbar">
        <select id="status-filter" aria-label="Filter by status">
          <option value="">All statuses</option>
          <option value="backlog">Backlog</option>
          <option value="queued">Queued</option>
          <option value="printing">Printing</option>
          <option value="blocked">Blocked</option>
          <option value="done">Done</option>
          <option value="cancelled">Cancelled</option>
        </select>
        <button id="refresh-queue" class="secondary" type="button">Refresh</button>
      </div>
      <div id="queue-list" class="queue-list"></div>
      <aside class="detail-panel">
        <div class="section-head">
          <h2>Inspection</h2>
          <button id="close-detail" class="secondary ${state.selectedItemId ? "" : "hidden"}" type="button">Close</button>
        </div>
        <div id="item-detail" class="detail-empty">
          <p class="muted">Select a queue item.</p>
        </div>
      </aside>
    </section>
  `;

  const filter = root.querySelector("#status-filter");
  const list = root.querySelector("#queue-list");
  const detail = root.querySelector("#item-detail");
  const close = root.querySelector("#close-detail");

  const paintList = () => {
    const items = filter.value ? state.queueItems.filter((item) => item.status === filter.value) : state.queueItems;
    list.innerHTML = items.length ? items.map((item) => renderQueueCard(item, state.selectedItemId)).join("") : `<p class="muted">No prints match this view.</p>`;
    list.querySelectorAll("[data-item-id]").forEach((button) => {
      button.addEventListener("click", () => loadItemDetail(button.dataset.itemId, detail, close, paintList));
    });
  };

  filter.addEventListener("change", paintList);
  root.querySelector("#refresh-queue").addEventListener("click", async () => {
    await refreshData();
    paintList();
    if (state.selectedItemId) await loadItemDetail(state.selectedItemId, detail, close, paintList);
  });
  close.addEventListener("click", () => {
    state.selectedItemId = null;
    close.classList.add("hidden");
    detail.className = "detail-empty";
    detail.innerHTML = `<p class="muted">Select a queue item.</p>`;
    paintList();
  });

  paintList();
  if (state.selectedItemId) loadItemDetail(state.selectedItemId, detail, close, paintList);
}

async function loadItemDetail(id, detail, close, paintList) {
  state.selectedItemId = Number(id);
  close.classList.remove("hidden");
  detail.className = "detail";
  detail.innerHTML = `<p class="muted">Loading item details...</p>`;
  paintList();
  try {
    const item = await apiClient.queueItem(state.selectedItemId);
    detail.innerHTML = renderDetail(item);
    detail.querySelector("#detail-form").addEventListener("submit", saveItemDetail);
    detail.querySelector("#note-form").addEventListener("submit", addItemNote);
  } catch (error) {
    detail.innerHTML = `<p class="muted">${escapeHTML(error.message)}</p>`;
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
      <button type="submit">Update item</button>
      <p id="detail-status" class="form-status"></p>
    </form>
    <section class="subsection">
      <h3>Notes</h3>
      <form id="note-form">
        <textarea name="body" rows="3" required placeholder="Add a comment..."></textarea>
        <button type="submit">Post note</button>
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

async function saveItemDetail(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const statusEl = form.querySelector("#detail-status");
  const payload = Object.fromEntries(new FormData(form).entries());
  payload.quantity = Number(payload.quantity || 1);
  statusEl.textContent = "Saving...";
  try {
    await apiClient.updateQueueItem(state.selectedItemId, payload);
    statusEl.textContent = "Saved.";
    await refreshData();
    renderQueue(document.querySelector("#route-view"), { item: state.selectedItemId });
  } catch (error) {
    statusEl.textContent = error.message;
  }
}

async function addItemNote(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const payload = Object.fromEntries(new FormData(form).entries());
  await apiClient.addQueueItemNote(state.selectedItemId, payload);
  form.reset();
  renderQueue(document.querySelector("#route-view"), { item: state.selectedItemId });
}
