import { escapeAttr, escapeHTML } from "./utils.js";

export function renderItemThumbnail(item, size = "") {
  const link = preferredThumbnailLink(item.links || []);
  const classes = ["thumbnail", size].filter(Boolean).join(" ");
  if (link?.thumbnail_status === "ready") {
    const alt = link.title || item.title || "Model preview";
    return `
      <div class="${classes}">
        <img src="/api/link-thumbnails/${escapeAttr(link.id)}" alt="${escapeAttr(alt)}" loading="lazy">
      </div>
    `;
  }

  const source = link?.source_type || firstSource(item.links || []) || "model";
  const status = link?.thumbnail_status || ((item.links || []).length ? "pending" : "none");
  return `
    <div class="${classes} thumbnail-fallback" aria-label="${escapeAttr(thumbnailLabel(link))}">
      <span>${escapeHTML(sourceLabel(source))}</span>
      <small>${escapeHTML(statusLabel(status))}</small>
    </div>
  `;
}

export function renderThumbnailStatus(item) {
  const link = preferredThumbnailLink(item.links || []);
  if (!link) return "";
  const status = link.thumbnail_status || "pending";
  const title = link.thumbnail_error || statusLabel(status);
  return `<span class="thumb-status thumb-${escapeAttr(status)}" title="${escapeAttr(title)}">${escapeHTML(statusLabel(status))}</span>`;
}

function preferredThumbnailLink(links) {
  return links.find((link) => link.thumbnail_status === "ready") ||
    links.find((link) => link.thumbnail_status && link.thumbnail_status !== "unsupported") ||
    links.find((link) => link.thumbnail_status);
}

function firstSource(links) {
  const link = links.find((item) => item.source_type);
  return link?.source_type;
}

function thumbnailLabel(link) {
  if (!link) return "No preview link";
  if (link.thumbnail_error) return `Thumbnail ${link.thumbnail_status}: ${link.thumbnail_error}`;
  return `Thumbnail ${link.thumbnail_status || "pending"}`;
}

function statusLabel(status) {
  const labels = {
    ready: "Preview ready",
    pending: "Preview pending",
    unavailable: "Preview unavailable",
    unsupported: "Preview unsupported",
    none: "No preview",
  };
  return labels[status] || "Preview unknown";
}

function sourceLabel(source) {
  const labels = {
    thingiverse: "Thingiverse",
    printables: "Printables",
    makerworld: "MakerWorld",
    github: "GitHub",
    direct: "Direct",
    other: "Model",
    model: "Model",
  };
  return labels[source] || "Model";
}
