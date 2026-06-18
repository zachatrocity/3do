import { apiClient } from "../api.js";
import { refreshData } from "../data.js";

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
        <label>
          Files
          <input name="files" type="file" multiple accept=".stl,.3mf,.gcode,.step,.stp,.obj,.zip,.png,.jpg,.jpeg,.webp">
        </label>
        <label>
          Notes
          <textarea name="description" rows="4"></textarea>
        </label>
        <button type="submit">Add to queue</button>
        <p id="form-status" class="form-status"></p>
      </form>
    </section>
  `;

  root.querySelector("#item-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const status = root.querySelector("#form-status");
    status.textContent = "Saving print request...";
    try {
      await apiClient.createQueueItem(new FormData(form));
      form.reset();
      status.textContent = "Added to queue. Thumbnail discovery will show status on the queue cards.";
      await refreshData();
    } catch (error) {
      status.textContent = error.message;
    }
  });
}
