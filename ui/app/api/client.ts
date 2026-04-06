import createClient from "openapi-fetch";
import type { paths } from "./types.gen";

const apiClient = createClient<paths>({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
});

export default apiClient;

