import { apiClient } from "./api.js";
import { isAdmin, state } from "./state.js";

export async function refreshData() {
  const requests = [apiClient.queueItems()];
  if (isAdmin()) requests.push(apiClient.printers(), apiClient.users());
  const [items, printers, users] = await Promise.all(requests);
  state.queueItems = items || [];
  state.printers = isAdmin() ? printers || [] : [];
  state.users = isAdmin() ? users || [] : [];
}
