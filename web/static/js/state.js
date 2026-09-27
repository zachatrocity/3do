export const state = {
  currentUser: null,
  queueItems: [],
  printers: [],
  users: [],
};

export function isAdmin() {
  return state.currentUser?.role === "admin";
}

export function setSessionUser(user) {
  state.currentUser = user;
}

export function clearSession() {
  state.currentUser = null;
  state.queueItems = [];
  state.printers = [];
  state.users = [];
}
