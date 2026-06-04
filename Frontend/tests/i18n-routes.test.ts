import assert from "node:assert/strict";
import { existsSync } from "node:fs";

const routePairs = [
  ["src/pages/404.astro", "src/pages/ru/404.astro"],
  ["src/pages/admin/moderation/index.astro", "src/pages/ru/admin/moderation/index.astro"],
  ["src/pages/api-reference.astro", "src/pages/ru/api-reference.astro"],
  ["src/pages/catalog/index.astro", "src/pages/ru/catalog/index.astro"],
  ["src/pages/catalog/[category].astro", "src/pages/ru/catalog/[category].astro"],
  ["src/pages/content/[id].astro", "src/pages/ru/content/[id].astro"],
  ["src/pages/index.astro", "src/pages/ru/index.astro"],
  ["src/pages/login.astro", "src/pages/ru/login.astro"],
  ["src/pages/profile/index.astro", "src/pages/ru/profile/index.astro"],
  ["src/pages/upload/index.astro", "src/pages/ru/upload/index.astro"],
  ["src/pages/users/[username].astro", "src/pages/ru/users/[username].astro"],
  ["src/pages/verify-email.astro", "src/pages/ru/verify-email.astro"],
];

for (const [englishRoute, russianRoute] of routePairs) {
  assert.equal(existsSync(englishRoute), true, `${englishRoute} must exist`);
  assert.equal(existsSync(russianRoute), true, `${russianRoute} must exist`);
}
