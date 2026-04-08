import createClient from "openapi-fetch";
import type { paths } from "./types.gen";

const apiClient = createClient<paths>({
  baseUrl: "/",
});

export default apiClient;
