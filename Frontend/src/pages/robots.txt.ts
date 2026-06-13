import type { APIRoute } from "astro";

const getRobotsTxt = (sitemapURL: URL) => [
  "User-agent: *",
  "Allow: /",
  `Sitemap: ${sitemapURL.href}`,
].join("\n");

export const GET: APIRoute = ({ site }) => {
  const origin = site ?? new URL("http://localhost:4321");
  const sitemapURL = new URL("sitemap-index.xml", origin);
  return new Response(getRobotsTxt(sitemapURL), {
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
    },
  });
};
