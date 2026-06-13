import type { APIListResponse, CatalogFilter, PublicContentItem } from "../api/types";

const contentSitemapPageLimit = 50;
const contentSitemapMaxItems = 45_000;
const locales = [
  { lang: "en", pathPrefix: "" },
  { lang: "ru", pathPrefix: "/ru" },
] as const;

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
  const urls = items.flatMap((item) => contentURLItems(item, site));
  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">',
    ...urls,
    "</urlset>",
  ].join("\n");
}

function contentURLItems(item: PublicContentItem, site: URL): string[] {
  const encodedID = encodeURIComponent(item.id);
  const links = locales.map((locale) => ({
    lang: locale.lang,
    href: absoluteURL(`${locale.pathPrefix}/content/${encodedID}`, site),
  }));
  const lastmod = sitemapDate(item.updated_at || item.published_at || item.created_at);

  return links.map((current) => [
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
