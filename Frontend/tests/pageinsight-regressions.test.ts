import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const layout = readFileSync("src/layouts/BaseLayout.astro", "utf8");
const baseCSS = readFileSync("src/styles/base.css", "utf8");
const siteMotion = readFileSync("src/scripts/site-motion.ts", "utf8");
const astroConfig = readFileSync("astro.config.mjs", "utf8");

assert.doesNotMatch(layout, /<style\s+is:inline\b/, "BaseLayout must not ship manual inline styles that Astro CSP does not hash");
assert.doesNotMatch(layout, /<script\s+is:inline\b/, "BaseLayout must not ship manual inline scripts that Astro CSP does not hash");
assert.match(layout, /<html\s+lang=\{locale\}\s+class="beeba-motion">/, "motion class should be rendered in SSR HTML");
assert.match(baseCSS, /@font-face\s*\{[\s\S]*font-family:\s*"Varela"/, "font face should live in hashed project CSS");

assert.match(layout, /const uploadLoginPath = `\$\{loginPath\}\?next=\$\{encodeURIComponent\(uploadPath\)\}`;/);
assert.match(layout, /href=\{authUser \? uploadPath : uploadLoginPath\}/);
assert.match(layout, /data-login-href=\{uploadLoginPath\}/);
assert.doesNotMatch(layout, /href=\{authUser \? uploadPath : undefined\}/);
assert.doesNotMatch(layout, /aria-disabled=\{authUser \? "false" : "true"\}/);

assert.match(siteMotion, /const uploadLoginHref = upload\?\.dataset\.loginHref \?\? "";/);
assert.match(siteMotion, /upload\.setAttribute\("href", isAuthenticated \? uploadHref : uploadLoginHref\);/);
assert.doesNotMatch(siteMotion, /upload\.removeAttribute\("href"\)/);

const cspDirectives = astroConfig.match(/directives:\s*\[[\s\S]*?\],\s*\},/)?.[0] ?? "";
assert.match(astroConfig, /const connectSrc = \[/, "CSP connect-src should be built separately from directives");
assert.match(astroConfig, /localHostnames\.has\(siteHostname\)/, "localhost CSP sources must be gated to local site URLs");
assert.match(cspDirectives, /connectSrc/, "Astro CSP directives should use computed connect-src");
assert.doesNotMatch(cspDirectives, /http:\/\/localhost:\*|http:\/\/127\.0\.0\.1:\*/, "prod CSP directives must not hardcode localhost sources");
