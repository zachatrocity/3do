import { isAdmin } from "./state.js";
import { renderDashboard } from "./views/dashboard.js";
import { renderSubmit } from "./views/submit.js";
import { renderQueue } from "./views/queue.js";
import { renderPrinters } from "./views/printers.js";
import { renderUsers } from "./views/users.js";

const routes = {
  dashboard: {
    label: "Dashboard",
    eyebrow: "Queue telemetry",
    title: "Dashboard",
    description: "Live queue load, blockers, printer activity, and recent completions.",
    render: renderDashboard,
  },
  submit: {
    label: "Submit print",
    eyebrow: "New request",
    title: "Submit print",
    description: "Capture source links, files, materials, owner, and print timing.",
    render: renderSubmit,
  },
  "admin-queue": {
    label: "Admin queue",
    eyebrow: "Operator board",
    title: "Admin queue",
    description: "Inspect jobs, move status, record notes, and review thumbnail health.",
    admin: true,
    render: renderQueue,
  },
  "admin-printers": {
    label: "Admin printers",
    eyebrow: "Machine rail",
    title: "Admin printers",
    description: "Track printer readiness, location, capabilities, and notes.",
    admin: true,
    render: renderPrinters,
  },
  "admin-users": {
    label: "Admin users",
    eyebrow: "Access control",
    title: "Admin users",
    description: "Create operators and manage account status.",
    admin: true,
    render: renderUsers,
  },
};

let viewRoot;
let headerRoot;
let navRoot;

export function setupRouter({ view, header, nav }) {
  viewRoot = view;
  headerRoot = header;
  navRoot = nav;
  window.addEventListener("hashchange", renderCurrentRoute);
}

export function renderCurrentRoute() {
  const { name, params } = parseHash();
  const route = routeFor(name);
  paintNav(route);
  paintHeader(route);
  if (route.admin && !isAdmin()) {
    viewRoot.innerHTML = `<section class="empty-state"><h2>Admin access required</h2><p class="muted">This route is only available to admin users.</p></section>`;
    return;
  }
  route.render(viewRoot, params);
}

export function routeTo(name, params = {}) {
  const search = new URLSearchParams(params).toString();
  window.location.hash = search ? `${name}?${search}` : name;
}

export function ensureDefaultRoute() {
  if (!window.location.hash || window.location.hash === "#") {
    routeTo("dashboard");
  } else {
    renderCurrentRoute();
  }
}

function parseHash() {
  const raw = window.location.hash.replace(/^#/, "") || "dashboard";
  const [name, query = ""] = raw.split("?");
  return {
    name,
    params: Object.fromEntries(new URLSearchParams(query).entries()),
  };
}

function routeFor(name) {
  if (routes[name]) return routes[name];
  return routes.dashboard;
}

function paintHeader(route) {
  headerRoot.innerHTML = `
    <div>
      <span>${route.eyebrow}</span>
      <h1>${route.title}</h1>
      <p>${route.description}</p>
    </div>
  `;
}

function paintNav(activeRoute) {
  navRoot.innerHTML = Object.entries(routes)
    .filter(([, route]) => !route.admin || isAdmin())
    .map(([name, route]) => `
      <a class="${route === activeRoute ? "active" : ""}" href="#${name}">
        <span>${route.label}</span>
      </a>
    `).join("");
}
