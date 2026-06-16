import type { AuthSession, PublicUser } from "../api/types";

export const AUTH_USER_KEY = "beeba.publicUser";

export interface BrowserAuthSession {
  user: PublicUser;
}

export function readAuthSession(): BrowserAuthSession | null {
  if (typeof window === "undefined") return null;
  try {
    const rawUser = window.sessionStorage.getItem(AUTH_USER_KEY);
    const user = rawUser ? (JSON.parse(rawUser) as PublicUser) : null;
    if (!user?.username) return null;
    return { user };
  } catch {
    return null;
  }
}

export function publishAuthSession(session: AuthSession) {
  if (typeof window === "undefined") return;
  publishSessionUser(session.user);
}

export function publishSessionUser(user: PublicUser) {
  if (typeof window === "undefined") return;
  window.sessionStorage.setItem(AUTH_USER_KEY, JSON.stringify(user));
  window.dispatchEvent(new CustomEvent("beeba:session-changed"));
}

export function clearAuthSession() {
  if (typeof window === "undefined") return;
  window.sessionStorage.removeItem(AUTH_USER_KEY);
  window.dispatchEvent(new CustomEvent("beeba:session-changed"));
}
