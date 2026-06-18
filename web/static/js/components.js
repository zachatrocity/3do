import { escapeAttr, escapeHTML, formatBytes, formatDateTime, itemSubtitle } from "./utils.js";
import { renderItemThumbnail, renderThumbnailStatus } from "./thumbnails.js";

export function metricCard(label, value, detail, tone = "") {
  return `
    <article class="metric-card ${tone}">
      <span class="metric-label">${escapeHTML(label)}</span>
      <strong>${escapeHTML(value)}</strong>
      <span class="metric-detail">${escapeHTML(detail)}</span>
    </article>
  `;
}

export function renderMiniQueueItem(item) {
  return `
    <button class="mini-item" type="button" data-item-id="${escapeAttr(item.id)}">
      <span>
        <strong>${escapeHTML(item.title)}</strong>
        <small>${escapeHTML(itemSubtitle(item))}</small>
      </span>
      <span class="badge status-${escapeAttr(item.status)}">${escapeHTML(item.status)}</span>
    </button>
  `;
}

export function renderQueueCard(item, selectedItemId) {
  return `
    <article class="queue-card${selectedItemId === item.id ? " selected" : ""}" data-card-id="${escapeAttr(item.id)}">
      ${renderItemThumbnail(item)}
      <div class="item-head">
        <div>
          <h3>${escapeHTML(item.title)}</h3>
          <p class="muted">${escapeHTML(item.description || "No notes")}</p>
        </div>
        <div class="badges">
          <span class="badge status-${escapeAttr(item.status)}">${escapeHTML(item.status)}</span>
          <span class="badge priority-${escapeAttr(item.priority)}">${escapeHTML(item.priority)}</span>
        </div>
      </div>
      <div class="meta">
        ${meta("Requester", item.requested_by)}
        ${meta("Owner", item.owner)}
        ${meta("Printer", item.printing_by)}
        ${meta("Material", item.material)}
        ${meta("Color", item.color)}
        ${meta("Qty", item.quantity)}
      </div>
      <div class="card-footer">
        ${renderThumbnailStatus(item)}
        <button class="secondary view-detail" type="button" data-item-id="${escapeAttr(item.id)}">Inspect</button>
      </div>
    </article>
  `;
}

export function renderLinks(links) {
  if (links.length === 0) return "";
  return `<div class="links">${links.map((link) => `
    <a href="${escapeAttr(link.url)}" target="_blank" rel="noreferrer">
      ${escapeHTML(link.source_type || "Source")}: ${escapeHTML(link.url)}
    </a>
    ${link.thumbnail_error ? `<small class="thumb-error">${escapeHTML(link.thumbnail_error)}</small>` : ""}
  `).join("")}</div>`;
}

export function renderFiles(files) {
  if (files.length === 0) return "";
  return `<div class="files">${files.map((file) => `<span>${escapeHTML(file.kind)}: ${escapeHTML(file.original_name)} (${formatBytes(file.size_bytes)})</span>`).join("")}</div>`;
}

export function renderNotes(notes) {
  if (notes.length === 0) return `<p class="muted">No notes yet.</p>`;
  return notes.map((note) => `
    <article class="timeline-entry">
      <span>${escapeHTML(note.author || "Unknown")} / ${escapeHTML(formatDateTime(note.created_at))}</span>
      <p>${escapeHTML(note.body)}</p>
    </article>
  `).join("");
}

export function renderStatusEvents(events) {
  if (events.length === 0) return `<p class="muted">No status history.</p>`;
  return events.map((event) => `
    <article class="timeline-entry">
      <span>${escapeHTML(event.old_status ? `${event.old_status} -> ${event.new_status}` : event.new_status)} / ${escapeHTML(formatDateTime(event.created_at))}</span>
      <p><strong>${escapeHTML(event.actor || "System")}:</strong> ${escapeHTML(event.note || "Status updated.")}</p>
    </article>
  `).join("");
}

export function renderPrinterPills(items) {
  if (items.length === 0) return `<p class="muted">No printers added.</p>`;
  return `<div class="printer-pills">${items.map((printer) => `
    <span>
      <strong>${escapeHTML(printer.name)}</strong>
      <em>${escapeHTML(printer.status || "unknown")}</em>
    </span>
  `).join("")}</div>`;
}

function meta(label, value) {
  if (!value) return "";
  return `<span><strong>${escapeHTML(label)}:</strong> ${escapeHTML(String(value))}</span>`;
}
