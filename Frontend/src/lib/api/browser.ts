import { getPublicAPIBaseURL } from "./client";

export async function browserJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const csrfHeader = csrfHeaders(init?.method);
  const response = await fetch(`${getPublicAPIBaseURL()}${path}`, {
    ...init,
    credentials: init?.credentials ?? "include",
    headers: {
      Accept: "application/json",
      ...csrfHeader,
      ...init?.headers,
    },
  });

  if (!response.ok) {
    throw new Error(await browserErrorMessage(response));
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

async function browserErrorMessage(response: Response): Promise<string> {
  try {
    const payload = await response.json();
    if (payload?.error?.message) {
      return payload.error.message;
    }
  } catch {
    // Keep stable generic message for non-JSON failures.
  }
  return `Request failed with status ${response.status}`;
}

export function authHeaders(token: string, headers: HeadersInit = {}): HeadersInit {
  const trimmed = token.trim();
  if (!trimmed) return headers;
  return {
    Authorization: `Bearer ${trimmed}`,
    ...headers,
  };
}

function csrfHeaders(method = "GET"): HeadersInit {
  if (!methodRequiresCSRF(method)) return {};
  const token = readCookie("beeba_csrf_token");
  return token ? { "X-CSRF-Token": token } : {};
}

function methodRequiresCSRF(method: string): boolean {
  return !["GET", "HEAD", "OPTIONS", "TRACE"].includes(method.toUpperCase());
}

function readCookie(name: string): string {
  if (typeof document === "undefined") return "";
  const prefix = `${name}=`;
  const cookie = document.cookie
    .split(";")
    .map((entry) => entry.trim())
    .find((entry) => entry.startsWith(prefix));
  return cookie ? decodeURIComponent(cookie.slice(prefix.length)) : "";
}
