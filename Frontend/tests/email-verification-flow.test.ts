import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const root = process.cwd();
const component = readFileSync(join(root, "src/components/auth/EmailVerificationApp.vue"), "utf8");
const authCSS = readFileSync(join(root, "src/styles/pages/auth.css"), "utf8");
const enPage = readFileSync(join(root, "src/pages/verify-email.astro"), "utf8");
const ruPage = readFileSync(join(root, "src/pages/ru/verify-email.astro"), "utf8");

assert.match(component, /profileHref:\s*string/);
assert.match(enPage, /profileHref="\/profile"/);
assert.match(ruPage, /profileHref="\/ru\/profile"/);

assert.doesNotMatch(component, /<section class="panel"/);
assert.match(component, /<section class="auth-page auth-page--verification"/);
assert.match(component, /<div class="auth-card auth-card--verification"/);
assert.match(authCSS, /\.auth-page--verification\s*\{[\s\S]*align-items:\s*start;/);
assert.match(authCSS, /\.auth-card--verification\s*\{[\s\S]*width:\s*min\(100%,\s*560px\);/);

assert.match(component, /import \{ navigateWithPageProgress \} from "\.\.\/\.\.\/lib\/ui\/page-progress";/);
assert.match(component, /navigateWithPageProgress\(props\.profileHref,\s*"replace"\);/);
