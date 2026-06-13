import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

const root = process.cwd();
const packageJSON = JSON.parse(readFileSync(join(root, "package.json"), "utf8")) as {
  dependencies?: Record<string, string>;
};
const astroConfig = readFileSync(join(root, "astro.config.mjs"), "utf8");
const baseLayout = readFileSync(join(root, "src/layouts/BaseLayout.astro"), "utf8");

assert.ok(packageJSON.dependencies?.["@astrojs/sitemap"], "@astrojs/sitemap dependency is required");
assert.match(astroConfig, /from '@astrojs\/sitemap'/);
assert.match(astroConfig, /sitemap\(/);
assert.match(astroConfig, /customSitemaps/);
assert.match(astroConfig, /content-sitemap\.xml/);
assert.ok(existsSync(join(root, "src/pages/robots.txt.ts")), "robots.txt Astro endpoint is required");
assert.ok(existsSync(join(root, "src/pages/content-sitemap.xml.ts")), "runtime content sitemap endpoint is required");
assert.match(baseLayout, /rel="sitemap"/);
assert.ok(existsSync(join(root, "public/OG-Image.png")), "Open Graph image should be served from public/");
assert.match(baseLayout, /const socialImage = new URL\("\/OG-Image\.png", site\);/);
assert.match(baseLayout, /<meta property="og:image" content=\{socialImage\.href\} \/>/);
assert.match(baseLayout, /<meta property="og:image:secure_url" content=\{socialImage\.href\} \/>/);
assert.match(baseLayout, /<meta property="og:image:type" content="image\/png" \/>/);
assert.match(baseLayout, /<meta property="og:image:width" content="1731" \/>/);
assert.match(baseLayout, /<meta property="og:image:height" content="909" \/>/);
assert.match(baseLayout, /<meta property="og:image:alt" content="BeeBa Basis VR content catalog" \/>/);
assert.match(baseLayout, /<meta name="twitter:image" content=\{socialImage\.href\} \/>/);
assert.match(baseLayout, /<meta name="twitter:image:alt" content="BeeBa Basis VR content catalog" \/>/);
