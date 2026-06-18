export function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#039;",
  })[char]);
}

export function escapeAttr(value) {
  return escapeHTML(value ?? "");
}

export function formPayload(form) {
  return Object.fromEntries(new FormData(form).entries());
}

export function userPayloadFromForm(form) {
  const formData = new FormData(form);
  const payload = Object.fromEntries(formData.entries());
  payload.active = formData.get("active") === "on";
  return payload;
}

export function formatBytes(value) {
  if (!value) return "0 B";
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

export function formatDateInput(value) {
  if (!value) return "";
  return String(value).slice(0, 10);
}

export function formatDateTime(value) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return date.toLocaleString();
}

export function formatDue(value) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return `Due ${date.toLocaleDateString()}`;
}

export function sortRecent(items) {
  return [...items].sort((a, b) => new Date(b.updated_at || b.created_at) - new Date(a.updated_at || a.created_at));
}

export function itemSubtitle(item) {
  return [item.priority, item.printing_by || item.owner, formatDue(item.due_at)].filter(Boolean).join(" / ");
}
