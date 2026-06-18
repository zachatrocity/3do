import { apiClient } from "../api.js";
import { refreshData } from "../data.js";
import { state } from "../state.js";
import { escapeAttr, userPayloadFromForm } from "../utils.js";

export function renderUsers(root) {
  root.innerHTML = `
    <section class="users-route">
      <form id="user-form" class="user-create-form">
        <input name="display_name" required placeholder="Display name" aria-label="Display name">
        <input name="email" type="email" required placeholder="email@example.com" aria-label="Email">
        <input name="password" type="password" required minlength="12" placeholder="Temporary password" aria-label="Temporary password">
        <select name="role" aria-label="Role">
          <option value="member">Member</option>
          <option value="admin">Admin</option>
        </select>
        <label class="checkbox-label">
          <input name="active" type="checkbox" checked>
          Active
        </label>
        <button type="submit">Add user</button>
      </form>
      <p id="user-status" class="form-status"></p>
      <div id="users" class="users">${renderUserRows()}</div>
    </section>
  `;

  root.querySelector("#user-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const status = root.querySelector("#user-status");
    status.textContent = "Saving...";
    try {
      await apiClient.createUser(userPayloadFromForm(form));
      form.reset();
      form.querySelector("[name='active']").checked = true;
      status.textContent = "User added.";
      await refreshData();
      attachUserRows(root);
    } catch (error) {
      status.textContent = error.message;
    }
  });

  attachUserRows(root);
}

function renderUserRows() {
  if (state.users.length === 0) return `<p class="muted">No users found.</p>`;
  return state.users.map((user) => `
    <form class="user-row" data-id="${escapeAttr(user.id)}">
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
    </form>
  `).join("");
}

function attachUserRows(root) {
  const users = root.querySelector("#users");
  users.innerHTML = renderUserRows();
  users.querySelectorAll(".user-row").forEach((row) => {
    row.addEventListener("submit", saveUser);
    row.querySelector(".delete-user").addEventListener("click", deleteUser);
  });
}

async function saveUser(event) {
  event.preventDefault();
  const status = document.querySelector("#user-status");
  const form = event.currentTarget;
  const payload = userPayloadFromForm(form);
  if (!payload.password) delete payload.password;
  status.textContent = "Saving...";
  try {
    await apiClient.updateUser(form.dataset.id, payload);
    status.textContent = "User saved.";
    await refreshData();
  } catch (error) {
    status.textContent = error.message;
  }
}

async function deleteUser(event) {
  const form = event.currentTarget.closest(".user-row");
  const name = form.querySelector("[name='display_name']").value || "this user";
  if (!confirm(`Delete ${name}?`)) return;
  const status = document.querySelector("#user-status");
  status.textContent = "Deleting...";
  try {
    await apiClient.deleteUser(form.dataset.id);
    status.textContent = "User deleted.";
    await refreshData();
    const root = document.querySelector("#route-view");
    attachUserRows(root);
  } catch (error) {
    status.textContent = error.message;
  }
}
