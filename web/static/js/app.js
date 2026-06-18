import { apiClient } from "./api.js";
import { refreshData } from "./data.js";
import { clearSession, setSessionUser, state } from "./state.js";
import { escapeHTML, formPayload } from "./utils.js";
import { ensureDefaultRoute, renderCurrentRoute, setupRouter } from "./router.js";

const appView = document.querySelector("#app-view");
const authView = document.querySelector("#auth-view");
const loginPanel = document.querySelector("#login-panel");
const bootstrapPanel = document.querySelector("#bootstrap-panel");
const sessionArea = document.querySelector("#session-area");
const loginForm = document.querySelector("#login-form");
const bootstrapForm = document.querySelector("#bootstrap-form");
const loginStatus = document.querySelector("#login-status");
const bootstrapStatus = document.querySelector("#bootstrap-status");

export async function startApp() {
  setupRouter({
    view: document.querySelector("#route-view"),
    header: document.querySelector("#route-header"),
    nav: document.querySelector("#route-nav"),
  });
  wireAuthForms();
  await loadSession();
}

async function loadSession() {
  try {
    const payload = await apiClient.session();
    setSessionUser(payload.user);
    await showApp();
  } catch (error) {
    clearSession();
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

async function showApp() {
  authView.classList.add("hidden");
  appView.classList.remove("hidden");
  renderSession();
  await refreshData();
  ensureDefaultRoute();
}

function renderSession() {
  sessionArea.innerHTML = `
    <div>
      <strong>${escapeHTML(state.currentUser.display_name)}</strong>
      <span>${escapeHTML(state.currentUser.role)}</span>
    </div>
    <button id="logout" class="secondary" type="button">Sign out</button>
  `;
  document.querySelector("#logout").addEventListener("click", logout);
}

function wireAuthForms() {
  loginForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    loginStatus.textContent = "Signing in...";
    try {
      const user = await apiClient.login(formPayload(loginForm));
      setSessionUser(user);
      loginForm.reset();
      loginStatus.textContent = "";
      await showApp();
    } catch (error) {
      loginStatus.textContent = error.message;
    }
  });

  bootstrapForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    bootstrapStatus.textContent = "Creating admin...";
    try {
      const user = await apiClient.bootstrapAdmin(formPayload(bootstrapForm));
      setSessionUser(user);
      bootstrapForm.reset();
      bootstrapStatus.textContent = "";
      await showApp();
    } catch (error) {
      bootstrapStatus.textContent = error.message;
    }
  });
}

async function logout() {
  await apiClient.logout();
  clearSession();
  renderCurrentRoute();
  await loadSession();
}
