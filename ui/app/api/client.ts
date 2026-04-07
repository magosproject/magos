import createClient from "openapi-fetch";
import type { paths } from "./types.gen";
import { apiBaseUrl } from "./base";

const apiClient = createClient<paths>({
  baseUrl: apiBaseUrl,
});

export default apiClient;
