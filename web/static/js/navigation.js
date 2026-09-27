// URL state is the single source of truth for queue navigation.
export const statuses = ["backlog", "queued", "printing", "blocked", "done", "cancelled"];

export function parseRoute(hash) {
  const raw = hash.replace(/^#/, "");
  let [name, query = ""] = raw.split("?");
  const params = Object.fromEntries(new URLSearchParams(query));
  if (!name || name === "dashboard") name = "queue";
  if (name === "admin-queue") {
    name = "queue";
    params.view ||= "list";
  }
  if (name === "queue") {
    params.view = params.view === "list" ? "list" : "board";
    if (!statuses.includes(params.status)) delete params.status;
    if (!/^[1-9]\d*$/.test(params.item || "")) delete params.item;
    return { name, params: Object.fromEntries(["view", "status", "item"].filter(key => params[key]).map(key => [key, params[key]])) };
  }
  return { name, params: {} };
}

export function routeHref(name, params = {}) {
  const search = new URLSearchParams(Object.entries(params).filter(([, value]) => value !== undefined && value !== null && value !== "")).toString();
  return `#${name}${search ? `?${search}` : ""}`;
}

export function queueParent(params) {
  const { item, ...parent } = params;
  return routeHref("queue", parent);
}
