import { apiClient } from "../api.js";
import { refreshData } from "../data.js";
import { state } from "../state.js";
import { escapeHTML, formPayload } from "../utils.js";

export function renderPrinters(root) {
  root.innerHTML = `
    <section class="admin-grid">
      <form id="printer-form" class="form-panel">
        <label>Name<input name="name" required placeholder="Prusa MK4"></label>
        <label>Location<input name="location" placeholder="Garage"></label>
        <label>Status<input name="status" placeholder="ready"></label>
        <label>Capabilities<input name="capabilities" placeholder="PLA, PETG, 0.4mm"></label>
        <label>Notes<textarea name="notes" rows="3"></textarea></label>
        <button type="submit">Add printer</button>
      </form>
      <div id="printers" class="printer-list">${renderPrinterList()}</div>
    </section>
  `;

  root.querySelector("#printer-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    await apiClient.createPrinter(formPayload(form));
    form.reset();
    await refreshData();
    root.querySelector("#printers").innerHTML = renderPrinterList();
  });
}

function renderPrinterList() {
  if (state.printers.length === 0) return `<p class="muted">No printers configured.</p>`;
  return state.printers.map((printer) => `
    <article class="printer-card">
      <div class="item-head">
        <div>
          <h3>${escapeHTML(printer.name)}</h3>
          <p class="muted">${escapeHTML(printer.location || "No location")}</p>
        </div>
        <span class="badge">${escapeHTML(printer.status || "unknown")}</span>
      </div>
      <p>${escapeHTML(printer.capabilities || "No capabilities listed.")}</p>
      ${printer.notes ? `<small>${escapeHTML(printer.notes)}</small>` : ""}
    </article>
  `).join("");
}
