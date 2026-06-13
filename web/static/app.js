const appView = document.querySelector("#app-view");
const authView = document.querySelector("#auth-view");
const loginPanel = document.querySelector("#login-panel");
const bootstrapPanel = document.querySelector("#bootstrap-panel");
const sessionArea = document.querySelector("#session-area");
const queueEl = document.querySelector("#queue");
const printersEl = document.querySelector("#printers");
const usersPanel = document.querySelector("#users-panel");
const usersEl = document.querySelector("#users");
const itemDetailEl = document.querySelector("#item-detail");
const closeDetailButton = document.querySelector("#close-detail");
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
let selectedItemId = null;

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
  if (selectedItemId) await loadItemDetail(selectedItemId);
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
    node.className = `item${selectedItemId === item.id ? " selected" : ""}`;
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
      <button class="secondary view-detail" type="button">Open detail</button>
    `;
    node.querySelector(".view-detail").addEventListener("click", () => loadItemDetail(item.id));
    queueEl.appendChild(node);
  }
}

async function loadItemDetail(id) {
  selectedItemId = Number(id);
  closeDetailButton.classList.remove("hidden");
  itemDetailEl.innerHTML = `<p class="muted">Loading item...</p>`;
  renderQueue();
  try {
    const item = await api(`/api/queue-items/${selectedItemId}`);
    renderItemDetail(item);
  } catch (error) {
    itemDetailEl.innerHTML = `<p class="muted">${escapeHTML(error.message)}</p>`;
  }
}

function renderItemDetail(item) {
  itemDetailEl.className = "detail";
  itemDetailEl.innerHTML = `
    <div class="detail-header">
      <div>
        <div class="item-title">${escapeHTML(item.title)}</div>
        <p class="muted">${escapeHTML(item.requested_by || "No requester")}</p>
      </div>
      <span class="badge status-${item.status}">${escapeHTML(item.status)}</span>
    </div>
    <p>${escapeHTML(item.description || "No notes on the request.")}</p>
    ${renderLinks(item.links || [])}
    ${renderFiles(item.files || [])}
    <form id="detail-form" class="detail-form">
      <div class="grid">
        ${selectField("status", "Status", item.status, ["backlog", "queued", "printing", "blocked", "done", "cancelled"])}
        ${selectField("priority", "Priority", item.priority, ["low", "normal", "high", "urgent"])}
      </div>
      <div class="grid">
        <label>Owner<input name="owner" value="${escapeAttr(item.owner)}"></label>
        <label>Printing by<input name="printing_by" value="${escapeAttr(item.printing_by)}"></label>
      </div>
      <div class="grid">
        <label>Material<input name="material" value="${escapeAttr(item.material)}"></label>
        <label>Color<input name="color" value="${escapeAttr(item.color)}"></label>
      </div>
      <div class="grid">
        <label>Quantity<input name="quantity" type="number" min="1" value="${escapeAttr(item.quantity || 1)}"></label>
        <label>Estimate<input name="estimated_minutes" type="number" min="1" value="${escapeAttr(item.estimated_minutes || "")}"></label>
      </div>
      <label>Due date<input name="due_at" type="date" value="${escapeAttr(formatDateInput(item.due_at))}"></label>
      <label>Status note<textarea name="status_note" rows="2" placeholder="Reason for status change"></textarea></label>
      <button type="submit">Save item</button>
      <p id="detail-status" class="form-status"></p>
    </form>
    <section class="subsection">
      <h3>Notes</h3>
      <form id="note-form">
        <textarea name="body" rows="3" required placeholder="Add a comment"></textarea>
        <button type="submit">Add note</button>
      </form>
      <div class="timeline">${renderNotes(item.notes || [])}</div>
    </section>
    <section class="subsection">
      <h3>Status history</h3>
      <div class="timeline">${renderStatusEvents(item.status_events || [])}</div>
    </section>
  `;
  document.querySelector("#detail-form").addEventListener("submit", saveItemDetail);
  document.querySelector("#note-form").addEventListener("submit", addItemNote);
}

function selectField(name, label, value, options) {
  return `<label>${label}<select name="${name}">${options.map((option) => (
    `<option value="${option}"${value === option ? " selected" : ""}>${option}</option>`
  )).join("")}</select></label>`;
}

function renderNotes(notes) {
  if (notes.length === 0) return `<p class="muted">No notes yet.</p>`;
  return notes.map((note) => `
    <article class="timeline-entry">
      <div><strong>${escapeHTML(note.author || "Unknown")}</strong> <span>${escapeHTML(formatDateTime(note.created_at))}</span></div>
      <p>${escapeHTML(note.body)}</p>
    </article>
  `).join("");
}

function renderStatusEvents(events) {
  if (events.length === 0) return `<p class="muted">No status history yet.</p>`;
  return events.map((event) => `
    <article class="timeline-entry">
      <div><strong>${escapeHTML(event.old_status ? `${event.old_status} -> ${event.new_status}` : event.new_status)}</strong> <span>${escapeHTML(formatDateTime(event.created_at))}</span></div>
      <p>${escapeHTML([event.actor, event.note].filter(Boolean).join(": ") || "Status recorded.")}</p>
    </article>
  `).join("");
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

function formatDateInput(value) {
  if (!value) return "";
  return String(value).slice(0, 10);
}

function formatDateTime(value) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return date.toLocaleString();
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

async function saveItemDetail(event) {
  event.preventDefault();
  const statusEl = document.querySelector("#detail-status");
  statusEl.textContent = "Saving...";
  const form = event.currentTarget;
  const formData = new FormData(form);
  const payload = Object.fromEntries(formData.entries());
  payload.quantity = Number(payload.quantity || 1);
  try {
    await api(`/api/queue-items/${selectedItemId}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    statusEl.textContent = "Saved.";
    await refresh();
  } catch (error) {
    statusEl.textContent = error.message;
  }
}

async function addItemNote(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const payload = Object.fromEntries(new FormData(form).entries());
  await api(`/api/queue-items/${selectedItemId}/notes`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  form.reset();
  await loadItemDetail(selectedItemId);
}

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
closeDetailButton.addEventListener("click", () => {
  selectedItemId = null;
  closeDetailButton.classList.add("hidden");
  itemDetailEl.className = "detail-empty";
  itemDetailEl.innerHTML = `<p class="muted">Select a queue item.</p>`;
  renderQueue();
});
document.querySelector("#refresh").addEventListener("click", () => {
  refresh().catch((error) => {
    queueEl.innerHTML = `<p class="muted">${escapeHTML(error.message)}</p>`;
  });
});

loadSession();
