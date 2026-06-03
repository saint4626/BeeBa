const PROTECTED_PATH_PATTERN = /^\/(?:admin|profile|upload)(?:\/|$)/;

export function getPostLogoutRedirectHref(location: Pick<Location | URL, "pathname" | "search" | "hash">): string | null {
  const normalizedPath = location.pathname.replace(/^\/ru(?=\/|$)/, "") || "/";
  if (!PROTECTED_PATH_PATTERN.test(normalizedPath)) {
    return null;
  }

  const loginPath = location.pathname === "/ru" || location.pathname.startsWith("/ru/") ? "/ru/login" : "/login";
  const next = `${location.pathname}${location.search}${location.hash}`;
  return `${loginPath}?next=${encodeURIComponent(next)}`;
}
