import { en } from "./i18n/locales/en";
import { ru } from "./i18n/locales/ru";
import type { Locale, UIStrings } from "./i18n/types";

export type { Locale, UIStrings } from "./i18n/types";

export const defaultLocale: Locale = "en";
export const supportedLocales = ["en", "ru"] as const;

export const ui: Record<Locale, UIStrings> = {
  en,
  ru,
};

export function normalizeLocale(locale: string | undefined): Locale {
  return locale === "ru" ? "ru" : defaultLocale;
}

export function localizedPath(locale: Locale, path: string): string {
  if (path.startsWith("http")) return path;
  const normalized = path.startsWith("/") ? path : `/${path}`;
  return locale === defaultLocale ? normalized : `/ru${normalized === "/" ? "" : normalized}`;
}

export function alternateLocale(locale: Locale): Locale {
  return locale === "ru" ? "en" : "ru";
}
