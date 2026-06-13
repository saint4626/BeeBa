import type { APIRoute } from "astro";
import { listPublicContent } from "../lib/api/content";
import { buildContentSitemapXML, collectContentSitemapItems } from "../lib/seo/content-sitemap";

export const prerender = false;

export const GET: APIRoute = async ({ site }) => {
  const sitemapSite = site ?? new URL("https://beeba.org/");
  const items = await collectContentSitemapItems(listPublicContent);
  const xml = buildContentSitemapXML(items, sitemapSite);

  return new Response(xml, {
    headers: {
      "content-type": "application/xml; charset=utf-8",
      "cache-control": "public, max-age=300, s-maxage=300",
    },
  });
};
