import assert from "node:assert/strict";
import { buildContentSitemapXML, collectContentSitemapItems } from "../src/lib/seo/content-sitemap.ts";
import type { APIListResponse, PublicContentItem } from "../src/lib/api/types.ts";

const first = contentItem("11111111-1111-4111-8111-111111111111", "2026-06-01T10:00:00.000Z");
const second = contentItem("22222222-2222-4222-8222-222222222222", "2026-06-02T12:00:00.000Z");
const calls: Array<{ limit: number; sort?: string; cursor?: string; include_nsfw?: boolean }> = [];

const collected = await collectContentSitemapItems(async (filter) => {
  calls.push(filter);
  const response: APIListResponse<PublicContentItem> = filter.cursor
    ? { data: [second], pagination: { limit: filter.limit, next_cursor: null } }
    : { data: [first], pagination: { limit: filter.limit, next_cursor: "next-page" } };
  return response;
});

assert.deepEqual(
  collected.map((item) => item.id),
  [first.id, second.id],
);
assert.deepEqual(calls, [
  { limit: 50, sort: "newest" },
  { limit: 50, sort: "newest", cursor: "next-page" },
]);

const xml = buildContentSitemapXML(collected, new URL("https://beeba.org/"));
assert.match(xml, /^<\?xml version="1.0" encoding="UTF-8"\?>/);
assert.match(xml, /<urlset[^>]+xmlns="http:\/\/www\.sitemaps\.org\/schemas\/sitemap\/0\.9"/);
assert.match(xml, /xmlns:xhtml="http:\/\/www\.w3\.org\/1999\/xhtml"/);
assert.match(xml, /<loc>https:\/\/beeba\.org\/content\/asset-11111111-11111111-1111-4111-8111-111111111111<\/loc>/);
assert.match(xml, /<xhtml:link rel="alternate" hreflang="ru" href="https:\/\/beeba\.org\/ru\/content\/asset-11111111-11111111-1111-4111-8111-111111111111"\/>/);
assert.match(xml, /<xhtml:link rel="alternate" hreflang="x-default" href="https:\/\/beeba\.org\/content\/asset-11111111-11111111-1111-4111-8111-111111111111"\/>/);
assert.match(xml, /<lastmod>2026-06-01T10:00:00.000Z<\/lastmod>/);
assert.match(xml, /<loc>https:\/\/beeba\.org\/ru\/content\/asset-22222222-22222222-2222-4222-8222-222222222222<\/loc>/);
assert.match(xml, /<loc>https:\/\/beeba\.org\/users\/beeba<\/loc>/);
assert.match(xml, /<loc>https:\/\/beeba\.org\/ru\/users\/beeba<\/loc>/);
assert.match(xml, /<xhtml:link rel="alternate" hreflang="x-default" href="https:\/\/beeba\.org\/users\/beeba"\/>/);
assert.doesNotMatch(xml, /\/login/);

function contentItem(id: string, updatedAt: string): PublicContentItem {
  return {
    id,
    slug: `asset-${id.slice(0, 8)}`,
    title: "Asset",
    description: "Public asset",
    status: "published",
    visibility: "public",
    nsfw: false,
    category: { slug: "worlds", name: "Worlds" },
    author: { id: "33333333-3333-3333-3333-333333333333", username: "beeba" },
    tags: [],
    likes_count: 0,
    downloads_count: 0,
    comments_count: 0,
    published_at: "2026-05-31T10:00:00.000Z",
    created_at: "2026-05-31T09:00:00.000Z",
    updated_at: updatedAt,
  };
}
