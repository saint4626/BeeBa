import type { APIListResponse, CatalogFilter, PublicContentItem } from "../api/types";
import { buildContentCanonicalPaths, buildLocaleAlternates } from "./content-url.ts";

const contentSitemapPageLimit = 50;
const contentSitemapMaxItems = 45_000;

type ContentPageFetcher = (filter: CatalogFilter) => Promise<APIListResponse<PublicContentItem>>;

export async function collectContentSitemapItems(fetchPage: ContentPageFetcher): Promise<PublicContentItem[]> {
  const items: PublicContentItem[] = [];
  let cursor: string | undefined;
  const seenCursors = new Set<string>();

  while (items.length < contentSitemapMaxItems) {
    const filter: CatalogFilter = {
      limit: contentSitemapPageLimit,
      sort: "newest",
    };
    if (cursor) {
      filter.cursor = cursor;
    }

    const page = await fetchPage(filter);
    const remaining = contentSitemapMaxItems - items.length;
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

export function buildContentSitemapXML(items: PublicContentItem[], site: URL): string {
  const urls = [
    ...items.flatMap((item) => contentURLItems(item, site)),
    ...profileURLItems(items, site),
  ];
  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">',
    ...urls,
    "</urlset>",
  ].join("\n");
}

function contentURLItems(item: PublicContentItem, site: URL): string[] {
  const paths = buildContentCanonicalPaths(item);
  const links = buildLocaleAlternates(paths.en, paths.ru).map((alternate) => ({
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

function profileURLItems(items: PublicContentItem[], site: URL): string[] {
  const authors = new Map<string, { username: string; updated_at?: string | null; published_at?: string | null; created_at?: string | null }>();
  for (const item of items) {
    if (!item.author.username || authors.has(item.author.username)) continue;
    authors.set(item.author.username, {
      username: item.author.username,
      updated_at: item.updated_at,
      published_at: item.published_at,
      created_at: item.created_at,
    });
  }

  return Array.from(authors.values()).flatMap((author) => {
    const encodedUsername = encodeURIComponent(author.username);
    const enPath = `/users/${encodedUsername}`;
    const ruPath = `/ru/users/${encodedUsername}`;
    const links = buildLocaleAlternates(enPath, ruPath).map((alternate) => ({
      lang: alternate.hreflang,
      href: absoluteURL(alternate.path, site),
    }));
    const lastmod = sitemapDate(author.updated_at || author.published_at || author.created_at);

    return links.filter((link) => link.lang !== "x-default").map((current) => [
      "  <url>",
      `    <loc>${escapeXML(current.href)}</loc>`,
      ...links.map((alternate) => `    <xhtml:link rel="alternate" hreflang="${alternate.lang}" href="${escapeXML(alternate.href)}"/>`),
      lastmod ? `    <lastmod>${escapeXML(lastmod)}</lastmod>` : "",
      "  </url>",
    ].filter(Boolean).join("\n"));
  });
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
