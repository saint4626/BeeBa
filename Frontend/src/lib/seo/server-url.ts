import type { Locale } from "../i18n";
import type { PublicServerItem } from "../api/types";
import { buildLocaleAlternates, type LocaleAlternatePath } from "./content-url";

const serverIDPattern = "[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}";
const serverIDAtEndPattern = new RegExp(`(?:^|-)(${serverIDPattern})$`, "i");
const maxServerMetaDescriptionLength = 160;

interface ServerURLParts {
  id: string;
  slug: string;
}

export type ServerAlternatePath = LocaleAlternatePath;

export function buildServerSlugParam(server: ServerURLParts): string {
  const slug = server.slug.trim() || "basis-server";
  return `${slug}-${server.id}`;
}

export function buildServerPath(locale: Locale, server: ServerURLParts): string {
  const prefix = locale === "ru" ? "/ru" : "";
  return `${prefix}/servers/${encodeURIComponent(buildServerSlugParam(server))}`;
}

export function buildServerCanonicalPaths(server: ServerURLParts): Record<"en" | "ru" | "xDefault", string> {
  const en = buildServerPath("en", server);
  const ru = buildServerPath("ru", server);
  return { en, ru, xDefault: en };
}

export function buildServerAlternates(enPath: string, ruPath: string): ServerAlternatePath[] {
  return buildLocaleAlternates(enPath, ruPath);
}

export function extractServerIDFromPathParam(param: string | undefined): string | null {
  if (!param) return null;
  const decodedParam = decodeURIComponent(param);
  const match = decodedParam.match(serverIDAtEndPattern);
  return match?.[1]?.toLowerCase() ?? null;
}

export function serverMetaDescription(server: PublicServerItem, locale: Locale): string {
  const cleanDescription = sanitizeServerMetaDescription(server.description || server.check.motd || "");
  if (cleanDescription) return cleanDescription;

  const owner = server.owner.display_name || server.owner.username;
  const online = onlineLabel(server, locale);
  if (locale === "ru") {
    return clampServerMetaDescription(`${server.name} - сервер Basis VR от ${owner} на BeeBa. ${online}`);
  }
  return clampServerMetaDescription(`${server.name} is a Basis VR server by ${owner} on BeeBa. ${online}`);
}

export function sanitizeServerMetaDescription(value: string | null | undefined): string {
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

  return clampServerMetaDescription(cleaned);
}

function onlineLabel(server: PublicServerItem, locale: Locale): string {
  const online = server.check.online_players;
  const max = server.check.max_players;
  if (typeof online === "number" && typeof max === "number") {
    return locale === "ru" ? `Онлайн ${online}/${max} игроков.` : `${online}/${max} players online.`;
  }
  if (server.check.status === "online") {
    return locale === "ru" ? "Сервер онлайн." : "The server is online.";
  }
  return locale === "ru" ? "Доступность проверяется." : "Availability is checked by BeeBa.";
}

function clampServerMetaDescription(value: string): string {
  if (value.length <= maxServerMetaDescriptionLength) return value;
  const truncated = value.slice(0, maxServerMetaDescriptionLength - 1);
  const lastSpace = truncated.lastIndexOf(" ");
  return `${truncated.slice(0, lastSpace > 120 ? lastSpace : truncated.length).trimEnd()}...`;
}
