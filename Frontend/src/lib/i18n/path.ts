export type I18nPathLocale = "en" | "ru";

export const defaultPathLocale: I18nPathLocale = "en";

export function localizedPath(locale: I18nPathLocale, path: string): string {
  if (path.startsWith("http")) return path;
  const normalizedInput = path.startsWith("/") ? path : `/${path}`;
  const parsed = splitPath(normalizedInput);
  const unprefixed = parsed.pathname.replace(/^\/ru(?=\/|$)/, "") || "/";
  const localized = locale === defaultPathLocale ? unprefixed : `/ru${unprefixed === "/" ? "" : unprefixed}`;
  return `${localized}${parsed.search}${parsed.hash}`;
}

function splitPath(path: string): { pathname: string; search: string; hash: string } {
  const hashIndex = path.indexOf("#");
  const beforeHash = hashIndex >= 0 ? path.slice(0, hashIndex) : path;
  const hash = hashIndex >= 0 ? path.slice(hashIndex) : "";
  const queryIndex = beforeHash.indexOf("?");
  return {
    pathname: queryIndex >= 0 ? beforeHash.slice(0, queryIndex) || "/" : beforeHash || "/",
    search: queryIndex >= 0 ? beforeHash.slice(queryIndex) : "",
    hash,
  };
}
