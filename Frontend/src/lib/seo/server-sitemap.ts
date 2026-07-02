import type { APIListResponse, PublicServerItem, ServerCatalogFilter } from "../api/types";
import { buildServerCanonicalPaths, buildServerAlternates } from "./server-url.ts";

const serverSitemapPageLimit = 50;
const serverSitemapMaxItems = 45_000;

type ServerPageFetcher = (filter: ServerCatalogFilter) => Promise<APIListResponse<PublicServerItem>>;

export async function collectServerSitemapItems(fetchPage: ServerPageFetcher): Promise<PublicServerItem[]> {
  const items: PublicServerItem[] = [];
  let cursor: string | undefined;
  const seenCursors = new Set<string>();

  while (items.length < serverSitemapMaxItems) {
    const filter: ServerCatalogFilter = {
      limit: serverSitemapPageLimit,
      sort: "updated",
    };
    if (cursor) {
      filter.cursor = cursor;
    }

    const page = await fetchPage(filter);
    const remaining = serverSitemapMaxItems - items.length;
    items.push(...page.data.slice(0, remaining));

    const nextCursor = page.pagination?.next_cursor ?? undefined;
    if (!nextCursor || page.data.length === 0 || seenCursors.has(nextCursor)) {
      break;
    }

    seenCursors.add(nextCursor);
    cursor = nextCursor;
  }

  return items;
}

export function buildServerSitemapXML(items: PublicServerItem[], site: URL): string {
  const urls = items.flatMap((item) => serverURLItems(item, site));
  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">',
    ...urls,
    "</urlset>",
  ].join("\n");
}

function serverURLItems(item: PublicServerItem, site: URL): string[] {
  const paths = buildServerCanonicalPaths(item);
  const links = buildServerAlternates(paths.en, paths.ru).map((alternate) => ({
    lang: alternate.hreflang,
    href: absoluteURL(alternate.path, site),
  }));
  const lastmod = sitemapDate(item.updated_at || item.published_at || item.created_at);

  return links.filter((link) => link.lang !== "x-default").map((current) => [
    "  <url>",
    `    <loc>${escapeXML(current.href)}</loc>`,
    ...links.map((alternate) => `    <xhtml:link rel="alternate" hreflang="${alternate.lang}" href="${escapeXML(alternate.href)}"/>`),
    lastmod ? `    <lastmod>${escapeXML(lastmod)}</lastmod>` : "",
    "  </url>",
  ].filter(Boolean).join("\n"));
}

function absoluteURL(path: string, site: URL): string {
  return new URL(path, site).href;
}

function sitemapDate(value?: string | null): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return date.toISOString();
}

function escapeXML(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&apos;");
}
