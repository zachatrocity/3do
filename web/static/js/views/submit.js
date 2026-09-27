import { apiClient } from "../api.js";
import { refreshData } from "../data.js";
import { routeTo } from "../router.js";

export function renderSubmit(root) {
  root.innerHTML = `
    <section class="form-panel submit-panel">
      <form id="item-form">
        <div class="form-grid wide">
          <label>
            Title
            <input name="title" required placeholder="Gridfinity bins">
          </label>
          <label>
            Links
            <textarea name="links" rows="3" placeholder="https://www.printables.com/..."></textarea>
          </label>
        </div>
        <label>
          Files
          <input name="files" type="file" multiple accept=".stl,.3mf,.gcode,.step,.stp,.obj,.zip,.png,.jpg,.jpeg,.webp">
        </label>
        <label>
          Notes
          <textarea name="description" rows="4"></textarea>
        </label>
        <details class="optional-fields"><summary>Print options — material, quantity, scheduling</summary>
        <div class="form-grid">
          <label>Status<select name="status">
            <option value="backlog">Backlog</option>
            <option value="queued" selected>Queued</option>
            <option value="printing">Printing</option>
            <option value="blocked">Blocked</option>
          </select></label>
          <label>Priority<select name="priority">
            <option value="normal" selected>Normal</option>
            <option value="low">Low</option>
            <option value="high">High</option>
            <option value="urgent">Urgent</option>
          </select></label>
          <label>Requested by<input name="requested_by" placeholder="Zach"></label>
          <label>Owner<input name="owner" placeholder="Shop"></label>
          <label>Material<input name="material" placeholder="PLA"></label>
          <label>Color<input name="color" placeholder="Black"></label>
          <label>Quantity<input name="quantity" type="number" min="1" value="1"></label>
          <label>Printing by<input name="printing_by" placeholder="A1 Mini"></label>
          <label>Due date<input name="due_at" type="date"></label>
          <label>Estimate<input name="estimated_minutes" type="number" min="1" placeholder="90"></label>
        </div>
        </details>
        <button type="submit">Add to queue</button>
        <p id="form-status" class="form-status" role="status"></p>
      </form>
    </section>
  `;

  root.querySelector("#item-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const status = root.querySelector("#form-status");
    const button = form.querySelector("button[type=submit]");
    button.disabled = true;
    status.textContent = "Saving print request...";
    try {
      const item = await apiClient.createQueueItem(new FormData(form));
      form.reset();
      status.textContent = "Print added.";
      try { await refreshData(); } catch { /* Detail can load independently. */ }
      if (form.isConnected) routeTo("queue", { view: "board", item: item.id }, { replace: true });
    } catch (error) {
      status.textContent = error.message;
    } finally { button.disabled = false; }
  });
}
