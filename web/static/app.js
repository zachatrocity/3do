const appView = document.querySelector("#app-view");
const authView = document.querySelector("#auth-view");
const loginPanel = document.querySelector("#login-panel");
const bootstrapPanel = document.querySelector("#bootstrap-panel");
const sessionArea = document.querySelector("#session-area");
const queueEl = document.querySelector("#queue");
const printersEl = document.querySelector("#printers");
const usersPanel = document.querySelector("#users-panel");
const usersEl = document.querySelector("#users");
const itemForm = document.querySelector("#item-form");
const printerForm = document.querySelector("#printer-form");
const userForm = document.querySelector("#user-form");
const loginForm = document.querySelector("#login-form");
const bootstrapForm = document.querySelector("#bootstrap-form");
const statusFilter = document.querySelector("#status-filter");
const formStatus = document.querySelector("#form-status");
const loginStatus = document.querySelector("#login-status");
const bootstrapStatus = document.querySelector("#bootstrap-status");
const userStatus = document.querySelector("#user-status");

let queueItems = [];
let currentUser = null;

async function api(path, options = {}) {
  const response = await fetch(path, options);
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(payload.error || response.statusText);
    error.status = response.status;
    error.payload = payload;
    throw error;
  }
  return payload;
}

async function loadSession() {
  try {
    const payload = await api("/api/session");
    currentUser = payload.user;
    showApp();
    await refresh();
  } catch (error) {
    currentUser = null;
    showAuth(Boolean(error.payload?.bootstrap_required));
  }
}

function showAuth(bootstrapRequired) {
  authView.classList.remove("hidden");
  appView.classList.add("hidden");
  loginPanel.classList.toggle("hidden", bootstrapRequired);
  bootstrapPanel.classList.toggle("hidden", !bootstrapRequired);
  sessionArea.innerHTML = "";
}

function showApp() {
  authView.classList.add("hidden");
  appView.classList.remove("hidden");
  usersPanel.classList.toggle("hidden", currentUser?.role !== "admin");
  sessionArea.innerHTML = `
    <span>${escapeHTML(currentUser.display_name)}</span>
    <button id="logout" type="button">Sign out</button>
  `;
  document.querySelector("#logout").addEventListener("click", logout);
}

async function refresh() {
  const requests = [
    api("/api/queue-items"),
    api("/api/printers"),
  ];
  if (currentUser?.role === "admin") {
    requests.push(api("/api/users"));
  }
  const [items, printers, users] = await Promise.all(requests);
  queueItems = items || [];
  renderQueue();
  renderPrinters(printers || []);
  if (currentUser?.role === "admin") renderUsers(users || []);
}

function renderQueue() {
  const filter = statusFilter.value;
  const items = filter ? queueItems.filter((item) => item.status === filter) : queueItems;
  queueEl.innerHTML = "";
  if (items.length === 0) {
    queueEl.innerHTML = `<p class="muted">No prints match this view.</p>`;
    return;
  }
  for (const item of items) {
    const node = document.createElement("article");
    node.className = "item";
    node.innerHTML = `
      <div class="item-head">
        <div>
          <div class="item-title">${escapeHTML(item.title)}</div>
          <p class="muted">${escapeHTML(item.description || "")}</p>
        </div>
        <div class="badges">
          <span class="badge status-${item.status}">${escapeHTML(item.status)}</span>
          <span class="badge">${escapeHTML(item.priority)}</span>
        </div>
      </div>
      <div class="meta">
        ${meta("Requested", item.requested_by)}
        ${meta("Owner", item.owner)}
        ${meta("Printing", item.printing_by)}
        ${meta("Material", item.material)}
        ${meta("Color", item.color)}
        ${meta("Qty", item.quantity)}
      </div>
      ${renderLinks(item.links || [])}
      ${renderFiles(item.files || [])}
    `;
    queueEl.appendChild(node);
  }
}

function renderPrinters(printers) {
  printersEl.innerHTML = "";
  if (printers.length === 0) {
    printersEl.innerHTML = `<p class="muted">No printers yet.</p>`;
    return;
  }
  for (const printer of printers) {
    const node = document.createElement("article");
    node.className = "printer";
    node.innerHTML = `
      <div class="item-head">
        <div>
          <div class="item-title">${escapeHTML(printer.name)}</div>
          <p class="muted">${escapeHTML(printer.location || "No location")}</p>
        </div>
        <span class="badge">${escapeHTML(printer.status)}</span>
      </div>
    `;
    printersEl.appendChild(node);
  }
}

function renderUsers(users) {
  usersEl.innerHTML = "";
  if (users.length === 0) {
    usersEl.innerHTML = `<p class="muted">No users yet.</p>`;
    return;
  }
  for (const user of users) {
    const node = document.createElement("form");
    node.className = "user-row";
    node.dataset.id = user.id;
    node.innerHTML = `
      <input name="display_name" required value="${escapeAttr(user.display_name)}" aria-label="Display name">
      <input name="email" type="email" required value="${escapeAttr(user.email)}" aria-label="Email">
      <select name="role" aria-label="Role">
        <option value="member"${user.role === "member" ? " selected" : ""}>Member</option>
        <option value="admin"${user.role === "admin" ? " selected" : ""}>Admin</option>
      </select>
      <input name="password" type="password" minlength="12" placeholder="New password" aria-label="New password">
      <label class="checkbox-label">
        <input name="active" type="checkbox"${user.active ? " checked" : ""}>
        Active
      </label>
      <button type="submit">Save</button>
      <button type="button" class="secondary delete-user">Delete</button>
    `;
    node.addEventListener("submit", saveUser);
    node.querySelector(".delete-user").addEventListener("click", deleteUser);
    usersEl.appendChild(node);
  }
}

function meta(label, value) {
  if (value === undefined || value === null || value === "") return "";
  return `<span><strong>${label}:</strong> ${escapeHTML(String(value))}</span>`;
}

function renderLinks(links) {
  if (links.length === 0) return "";
  return `<div class="links">${links.map((link) => `<a href="${escapeAttr(link.url)}" target="_blank" rel="noreferrer">${escapeHTML(link.source_type)}: ${escapeHTML(link.url)}</a>`).join("")}</div>`;
}

function renderFiles(files) {
  if (files.length === 0) return "";
  return `<div class="files">${files.map((file) => `<span>${escapeHTML(file.kind)} - ${escapeHTML(file.original_name)} - ${formatBytes(file.size_bytes)}</span>`).join("")}</div>`;
}

function formatBytes(value) {
  if (!value) return "0 B";
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

function escapeHTML(value) {
  return String(value).replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#039;",
  })[char]);
}

function escapeAttr(value) {
  return escapeHTML(value || "");
}

function payloadFromForm(form) {
  const formData = new FormData(form);
  const payload = Object.fromEntries(formData.entries());
  payload.active = formData.get("active") === "on";
  return payload;
}

async function logout() {
  await api("/api/logout", { method: "POST" });
  currentUser = null;
  await loadSession();
}

loginForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  loginStatus.textContent = "Signing in...";
  try {
    const payload = Object.fromEntries(new FormData(loginForm).entries());
    currentUser = await api("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    loginForm.reset();
    loginStatus.textContent = "";
    showApp();
    await refresh();
  } catch (error) {
    loginStatus.textContent = error.message;
  }
});

bootstrapForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  bootstrapStatus.textContent = "Creating admin...";
  try {
    const payload = Object.fromEntries(new FormData(bootstrapForm).entries());
    currentUser = await api("/api/bootstrap/admin", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    bootstrapForm.reset();
    bootstrapStatus.textContent = "";
    showApp();
    await refresh();
  } catch (error) {
    bootstrapStatus.textContent = error.message;
  }
});

itemForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  formStatus.textContent = "Saving...";
  try {
    const formData = new FormData(itemForm);
    await api("/api/queue-items", { method: "POST", body: formData });
    itemForm.reset();
    formStatus.textContent = "Added.";
    await refresh();
  } catch (error) {
    formStatus.textContent = error.message;
  }
});

printerForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const formData = new FormData(printerForm);
  const payload = Object.fromEntries(formData.entries());
  await api("/api/printers", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  printerForm.reset();
  await refresh();
});

userForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  userStatus.textContent = "Saving...";
  try {
    await api("/api/users", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payloadFromForm(userForm)),
    });
    userForm.reset();
    userForm.querySelector("[name='active']").checked = true;
    userStatus.textContent = "User added.";
    await refresh();
  } catch (error) {
    userStatus.textContent = error.message;
  }
});

async function saveUser(event) {
  event.preventDefault();
  userStatus.textContent = "Saving...";
  const form = event.currentTarget;
  const payload = payloadFromForm(form);
  if (!payload.password) delete payload.password;
  try {
    await api(`/api/users/${form.dataset.id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    userStatus.textContent = "User saved.";
    await refresh();
  } catch (error) {
    userStatus.textContent = error.message;
  }
}

async function deleteUser(event) {
  const form = event.currentTarget.closest(".user-row");
  const name = form.querySelector("[name='display_name']").value || "this user";
  if (!confirm(`Delete ${name}?`)) return;
  userStatus.textContent = "Deleting...";
  try {
    await api(`/api/users/${form.dataset.id}`, { method: "DELETE" });
    userStatus.textContent = "User deleted.";
    await refresh();
  } catch (error) {
    userStatus.textContent = error.message;
  }
}

statusFilter.addEventListener("change", renderQueue);
document.querySelector("#refresh").addEventListener("click", () => {
  refresh().catch((error) => {
    queueEl.innerHTML = `<p class="muted">${escapeHTML(error.message)}</p>`;
  });
});

loadSession();
