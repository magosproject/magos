export const csrfCookieName = "magos_csrf";
export const redirectToQueryParam = "redirectTo";

export interface AuthConfig {
  enabled: boolean;
  adminEnabled: boolean;
  oidc: {
    enabled: boolean;
    issuerUrl?: string;
    clientId?: string;
    loginUrl?: string;
  };
}

export interface Identity {
  provider: "admin" | "oidc" | "service";
  subject: string;
  username?: string;
  groups?: string[];
  isAdmin: boolean;
}

export interface MeResponse {
  authenticated: boolean;
  identity?: Identity;
}

export function csrfToken(): string | undefined {
  return document.cookie
    .split(";")
    .map((part) => part.trim())
    .find((part) => part.startsWith(`${csrfCookieName}=`))
    ?.split("=")
    .slice(1)
    .join("=");
}

export function isSafeRedirectPath(path: string | null): path is string {
  if (!path || !path.startsWith("/") || path.startsWith("//")) return false;
  try {
    const url = new URL(path, window.location.origin);
    return url.origin === window.location.origin;
  } catch {
    return false;
  }
}

export async function getAuthConfig(): Promise<AuthConfig> {
  const response = await fetch("/auth/config", { credentials: "include" });
  if (!response.ok) throw new Error("failed to load auth config");
  return response.json() as Promise<AuthConfig>;
}

export async function getMe(): Promise<MeResponse> {
  const response = await fetch("/auth/me", { credentials: "include" });
  if (response.status === 401) return { authenticated: false };
  if (!response.ok) throw new Error("failed to load current user");
  return response.json() as Promise<MeResponse>;
}

export async function adminLogin(password: string): Promise<MeResponse> {
  const response = await fetch("/auth/admin/login", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ password }),
  });
  if (!response.ok) {
    throw new Error(await errorMessage(response, "failed to sign in"));
  }
  return response.json() as Promise<MeResponse>;
}

export async function logout(): Promise<void> {
  const token = csrfToken();
  await fetch("/auth/logout", {
    method: "POST",
    credentials: "include",
    headers: token ? { "X-CSRF-Token": token } : undefined,
  });
}

async function errorMessage(response: Response, fallback: string): Promise<string> {
  try {
    const body: unknown = await response.json();
    if (body && typeof body === "object" && "error" in body) {
      const error = (body as { error?: unknown }).error;
      if (typeof error === "string" && error.trim() !== "") return error;
    }
  } catch {
    // Ignore malformed or non-JSON error bodies and use the generic fallback.
  }
  return fallback;
}

export function loginPath(redirectTo = window.location.pathname + window.location.search): string {
  const params = new URLSearchParams();
  if (isSafeRedirectPath(redirectTo)) {
    params.set(redirectToQueryParam, redirectTo);
  }
  return `/login${params.size > 0 ? `?${params.toString()}` : ""}`;
}
