import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const root = process.cwd();
const component = readFileSync(join(root, "src/components/auth/AccountActionConfirmationApp.vue"), "utf8");
const authCSS = readFileSync(join(root, "src/styles/pages/auth.css"), "utf8");
const enPasswordPage = readFileSync(join(root, "src/pages/confirm-password-change.astro"), "utf8");
const enEmailPage = readFileSync(join(root, "src/pages/confirm-email-change.astro"), "utf8");
const ruPasswordPage = readFileSync(join(root, "src/pages/ru/confirm-password-change.astro"), "utf8");
const ruEmailPage = readFileSync(join(root, "src/pages/ru/confirm-email-change.astro"), "utf8");

assert.match(component, /profileHref:\s*string/);
assert.match(component, /loginHref:\s*string/);
assert.match(component, /import \{ navigateWithPageProgress \} from "\.\.\/\.\.\/lib\/ui\/page-progress";/);

assert.doesNotMatch(component, /<section class="panel"/);
assert.match(component, /<section class="auth-page auth-page--account-action"/);
assert.match(component, /<div class="auth-card auth-card--account-action"/);
assert.match(component, /<form class="auth-form"/);
assert.match(component, /<label class="auth-field"/);
assert.match(component, /class="auth-submit"/);

assert.match(authCSS, /\.auth-page--account-action/);
assert.match(authCSS, /\.auth-card--account-action/);
assert.match(authCSS, /width:\s*min\(100%,\s*560px\);/);

assert.match(component, /navigateWithPageProgress\(props\.loginHref,\s*"replace"\);/);
assert.match(component, /navigateWithPageProgress\(props\.profileHref,\s*"replace"\);/);

assert.match(enPasswordPage, /profileHref="\/profile"/);
assert.match(enPasswordPage, /loginHref="\/login"/);
assert.match(enEmailPage, /profileHref="\/profile"/);
assert.match(enEmailPage, /loginHref="\/login"/);
assert.match(ruPasswordPage, /profileHref="\/ru\/profile"/);
assert.match(ruPasswordPage, /loginHref="\/ru\/login"/);
assert.match(ruEmailPage, /profileHref="\/ru\/profile"/);
assert.match(ruEmailPage, /loginHref="\/ru\/login"/);
