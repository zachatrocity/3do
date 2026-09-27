import { isAdmin, state } from "./state.js";
import { renderSubmit } from "./views/submit.js";
import { renderQueue } from "./views/queue.js";
import { renderPrinters } from "./views/printers.js";
import { renderUsers } from "./views/users.js";
import { parseRoute, routeHref, queueParent } from "./navigation.js";
import { escapeAttr } from "./utils.js";

const routes = {
  queue: { label: "Queue", description: "Track prints from request to completion.", render: renderQueue },
  submit: { label: "New print", description: "Add a model link or upload files to get started.", render: renderSubmit },
  "admin-printers": { label: "Printers", description: "Manage workshop printers.", admin: true, render: renderPrinters },
  "admin-users": { label: "People", description: "Manage accounts and access.", admin: true, render: renderUsers },
};
let viewRoot, headerRoot, navRoot;

export function setupRouter({ view, header, nav }) {
  viewRoot = view; headerRoot = header; navRoot = nav;
  window.addEventListener("hashchange", renderCurrentRoute);
}

export function renderCurrentRoute() {
  if (!state.currentUser) return;
  const { name, params } = parseRoute(window.location.hash);
  const route = routes[name];
  navRoot.innerHTML = Object.entries(routes).filter(([, r]) => !r.admin || isAdmin()).map(([key, r]) => `
    ${key === "admin-printers" ? '<span class="nav-group-label">Settings</span>' : ""}
    <a ${key === name ? 'class="active" aria-current="page"' : ""} href="#${key}">${r.label}</a>`).join("");
  if (!route) {
    document.title = "Page not found · 3do";
    headerRoot.innerHTML = `<h1 tabindex="-1">Page not found</h1>`;
    viewRoot.innerHTML = `<section class="empty-state"><p>This page doesn't exist.</p><a href="#queue">Return to queue</a></section>`;
    headerRoot.querySelector("h1").focus({ preventScroll: true });
    return;
  }
  const canonical = routeHref(name, params);
  if (window.location.hash !== canonical) history.replaceState(null, "", canonical);
  const detail = name === "queue" && params.item;
  const title = detail ? "Print details" : route.label;
  document.title = `${title} · 3do`;
  headerRoot.innerHTML = `<div>
    ${detail ? `<a class="back-link" href="${escapeAttr(queueParent(params))}">← Back to queue</a>` : ""}
    <h1 tabindex="-1">${title}</h1>
    ${detail ? "" : `<p>${route.description}</p>`}
  </div>`;
  if (route.admin && !isAdmin()) {
    viewRoot.innerHTML = `<section class="empty-state"><h2>Admin access required</h2><a href="#queue">Return to queue</a></section>`;
  } else {
    route.render(viewRoot, params);
  }
  headerRoot.querySelector("h1").focus({ preventScroll: true });
  window.scrollTo(0, 0);
}

export function routeTo(name, params = {}, { replace = false } = {}) {
  const hash = routeHref(name, params);
  if (replace) { history.replaceState(null, "", hash); renderCurrentRoute(); }
  else if (window.location.hash !== hash) window.location.hash = hash;
}
export function ensureDefaultRoute() { renderCurrentRoute(); }
