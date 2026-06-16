import createClient from "openapi-fetch";
import type { paths } from "./types.gen";
import { csrfToken, loginPath } from "./auth";

const authenticatedFetch: typeof fetch = async (input, init = {}) => {
  const method = init.method?.toUpperCase() ?? "GET";
  const headers = new Headers(init.headers);
  if (!["GET", "HEAD", "OPTIONS", "TRACE"].includes(method)) {
    const token = csrfToken();
    if (token) headers.set("X-CSRF-Token", token);
  }

  const response = await fetch(input, {
    ...init,
    headers,
    credentials: "include",
  });

  if (response.status === 401 && window.location.pathname !== "/login") {
    window.location.replace(loginPath());
  }
  return response;
};

const apiClient = createClient<paths>({
  baseUrl: "/",
  fetch: authenticatedFetch,
});

export default apiClient;
