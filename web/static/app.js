const queueEl = document.querySelector("#queue");
const printersEl = document.querySelector("#printers");
const itemForm = document.querySelector("#item-form");
const printerForm = document.querySelector("#printer-form");
const statusFilter = document.querySelector("#status-filter");
const formStatus = document.querySelector("#form-status");

let queueItems = [];

async function api(path, options = {}) {
  const response = await fetch(path, options);
  if (!response.ok) {
    const payload = await response.json().catch(() => ({}));
    throw new Error(payload.error || response.statusText);
  }
  return response.json();
}

async function refresh() {
  const [items, printers] = await Promise.all([
    api("/api/queue-items"),
    api("/api/printers"),
  ]);
  queueItems = items || [];
  renderQueue();
  renderPrinters(printers || []);
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
  return `<div class="files">${files.map((file) => `<span>${escapeHTML(file.kind)} · ${escapeHTML(file.original_name)} · ${formatBytes(file.size_bytes)}</span>`).join("")}</div>`;
}

function formatBytes(value) {
  if (!value) return "0 B";
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

function escapeHTML(value) {
  return value.replace(/[&<>"']/g, (char) => ({
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

statusFilter.addEventListener("change", renderQueue);
document.querySelector("#refresh").addEventListener("click", refresh);

refresh().catch((error) => {
  queueEl.innerHTML = `<p class="muted">${escapeHTML(error.message)}</p>`;
});
