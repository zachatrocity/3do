export async function api(path, options = {}) {
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

export const apiClient = {
  session: () => api("/api/session"),
  login: (payload) => api("/api/login", jsonOptions("POST", payload)),
  bootstrapAdmin: (payload) => api("/api/bootstrap/admin", jsonOptions("POST", payload)),
  logout: () => api("/api/logout", { method: "POST" }),
  queueItems: () => api("/api/queue-items"),
  queueItem: (id) => api(`/api/queue-items/${id}`),
  createQueueItem: (formData) => api("/api/queue-items", { method: "POST", body: formData }),
  updateQueueItem: (id, payload) => api(`/api/queue-items/${id}`, jsonOptions("PATCH", payload)),
  addQueueItemNote: (id, payload) => api(`/api/queue-items/${id}/notes`, jsonOptions("POST", payload)),
  printers: () => api("/api/printers"),
  createPrinter: (payload) => api("/api/printers", jsonOptions("POST", payload)),
  users: () => api("/api/users"),
  createUser: (payload) => api("/api/users", jsonOptions("POST", payload)),
  updateUser: (id, payload) => api(`/api/users/${id}`, jsonOptions("PATCH", payload)),
  deleteUser: (id) => api(`/api/users/${id}`, { method: "DELETE" }),
};

function jsonOptions(method, payload) {
  return {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  };
}
