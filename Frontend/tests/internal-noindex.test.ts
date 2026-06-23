import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const internalPages = [
  "src/pages/login.astro",
  "src/pages/ru/login.astro",
  "src/pages/verify-email.astro",
  "src/pages/ru/verify-email.astro",
  "src/pages/confirm-email-change.astro",
  "src/pages/ru/confirm-email-change.astro",
  "src/pages/confirm-password-change.astro",
  "src/pages/ru/confirm-password-change.astro",
  "src/pages/profile/index.astro",
  "src/pages/ru/profile/index.astro",
  "src/pages/upload/index.astro",
  "src/pages/ru/upload/index.astro",
  "src/pages/admin/moderation/index.astro",
  "src/pages/ru/admin/moderation/index.astro",
  "src/pages/404.astro",
  "src/pages/ru/404.astro",
];

for (const pagePath of internalPages) {
  const source = readFileSync(pagePath, "utf8");
  assert.match(source, /robots="noindex,\s*nofollow"/, `${pagePath} must render noindex robots meta`);
}
