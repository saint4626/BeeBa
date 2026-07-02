import type { APIRoute } from "astro";
import { listPublicServers } from "../lib/api/servers";
import { buildServerSitemapXML, collectServerSitemapItems } from "../lib/seo/server-sitemap";

export const prerender = false;

export const GET: APIRoute = async ({ site }) => {
  const sitemapSite = site ?? new URL("https://beeba.org/");
  const items = await collectServerSitemapItems(listPublicServers);
  const xml = buildServerSitemapXML(items, sitemapSite);

  return new Response(xml, {
    headers: {
      "content-type": "application/xml; charset=utf-8",
      "cache-control": "public, max-age=300, s-maxage=300",
    },
  });
};
