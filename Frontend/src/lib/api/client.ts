import type { APIErrorResponse } from "./types";
import { PUBLIC_API_BASE_URL } from "astro:env/client";

const fallbackServerAPIBaseURL = "http://127.0.0.1:8088/api/v1";
const fallbackBrowserAPIBaseURL = PUBLIC_API_BASE_URL;

export function getServerAPIBaseURL(): string {
  return trimTrailingSlash(
    runtimeEnv().API_BASE_URL ??
      import.meta.env.API_BASE_URL ??
      import.meta.env.PUBLIC_API_BASE_URL ??
      serverFallbackAPIBaseURL(),
  );
}

function serverFallbackAPIBaseURL(): string {
  return fallbackBrowserAPIBaseURL.startsWith("/") ? fallbackServerAPIBaseURL : fallbackBrowserAPIBaseURL;
}

export function getPublicAPIBaseURL(): string {
  if (isBrowser()) {
    return browserAPIBaseURL(
      runtimeEnv().PUBLIC_API_BASE_URL ??
        import.meta.env.PUBLIC_API_BASE_URL ??
        fallbackBrowserAPIBaseURL,
    );
  }

  return trimTrailingSlash(
    runtimeEnv().PUBLIC_API_BASE_URL ??
      import.meta.env.PUBLIC_API_BASE_URL ??
      fallbackBrowserAPIBaseURL,
  );
}

export async function apiGet<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${getServerAPIBaseURL()}${path}`, {
    ...init,
    headers: {
      Accept: "application/json",
      ...init?.headers,
    },
  });

  if (!response.ok) {
    const message = await errorMessage(response);
    throw new Error(message);
  }

  return response.json() as Promise<T>;
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}

function isBrowser(): boolean {
  return typeof window !== "undefined";
}

function browserAPIBaseURL(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) return fallbackBrowserAPIBaseURL;

  try {
    const url = new URL(trimmed, window.location.origin);
    if (url.origin === window.location.origin || isLoopbackHost(url.hostname)) {
      return trimTrailingSlash(`${url.pathname}${url.search}`);
    }
    return trimTrailingSlash(url.toString());
  } catch {
    return trimTrailingSlash(trimmed);
  }
}

function isLoopbackHost(hostname: string): boolean {
  return hostname === "127.0.0.1" || hostname === "localhost" || hostname === "::1";
}

function runtimeEnv(): Record<string, string | undefined> {
  const globalWithProcess = globalThis as typeof globalThis & {
    process?: { env?: Record<string, string | undefined> };
  };
  return globalWithProcess.process?.env ?? {};
}

async function errorMessage(response: Response): Promise<string> {
  try {
    const payload = (await response.json()) as APIErrorResponse;
    if (payload.error?.message) {
      return payload.error.message;
    }
  } catch {
    // Keep a stable generic message if the backend returns a non-JSON error.
  }
  return `API request failed with status ${response.status}`;
}
