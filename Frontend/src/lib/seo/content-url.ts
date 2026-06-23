import type { Locale } from "../i18n";
import type { PublicContentItem } from "../api/types";

const contentIDPattern = "[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}";
const contentIDAtEndPattern = new RegExp(`(?:^|-)(${contentIDPattern})$`, "i");
const maxMetaDescriptionLength = 160;

interface ContentURLParts {
  id: string;
  slug: string;
}

export interface LocaleAlternatePath {
  hreflang: "en" | "ru" | "x-default";
  path: string;
}

export function buildContentSlugParam(content: ContentURLParts): string {
  const slug = content.slug.trim() || "asset";
  return `${slug}-${content.id}`;
}

export function buildContentPath(locale: Locale, content: ContentURLParts): string {
  const prefix = locale === "ru" ? "/ru" : "";
  return `${prefix}/content/${encodeURIComponent(buildContentSlugParam(content))}`;
}

export function buildContentCanonicalPaths(content: ContentURLParts): Record<"en" | "ru" | "xDefault", string> {
  const en = buildContentPath("en", content);
  const ru = buildContentPath("ru", content);
  return { en, ru, xDefault: en };
}

export function buildLocaleAlternates(enPath: string, ruPath: string): LocaleAlternatePath[] {
  return [
    { hreflang: "en", path: enPath },
    { hreflang: "ru", path: ruPath },
    { hreflang: "x-default", path: enPath },
  ];
}

export function extractContentIDFromPathParam(param: string | undefined): string | null {
  if (!param) return null;
  const decodedParam = decodeURIComponent(param);
  const match = decodedParam.match(contentIDAtEndPattern);
  return match?.[1]?.toLowerCase() ?? null;
}

export function contentMetaDescription(item: PublicContentItem, locale: Locale): string {
  const cleanDescription = sanitizeMetaDescription(item.description);
  if (cleanDescription) return cleanDescription;

  const author = item.author.display_name || item.author.username;
  if (locale === "ru") {
    return clampMetaDescription(`${item.title} - опубликованный .bee ассет Basis в категории ${item.category.name} от ${author} на BeeBa.`);
  }
  return clampMetaDescription(`${item.title} is a published Basis ${item.category.name} .bee asset by ${author} on BeeBa.`);
}

export function sanitizeMetaDescription(value: string | null | undefined): string {
  if (!value) return "";

  const cleaned = value
    .replace(/!\[([^\]]*)\]\([^)]+\)/g, "$1")
    .replace(/\[([^\]]+)\]\([^)]+\)/g, "$1")
    .replace(/`([^`]+)`/g, "$1")
    .replace(/\bhttps?:\/\/[^\s)]+/gi, "")
    .replace(/\bwww\.[^\s)]+/gi, "")
    .replace(/^[#>\s-]+/gm, "")
    .replace(/[*_~]+/g, "")
    .replace(/\s+([,.!?;:])/g, "$1")
    .replace(/\s+/g, " ")
    .trim();

  return clampMetaDescription(cleaned);
}

function clampMetaDescription(value: string): string {
  if (value.length <= maxMetaDescriptionLength) return value;
  const truncated = value.slice(0, maxMetaDescriptionLength - 1);
  const lastSpace = truncated.lastIndexOf(" ");
  return `${truncated.slice(0, lastSpace > 120 ? lastSpace : truncated.length).trimEnd()}...`;
}
