import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const drawer = readFileSync("src/components/profile/OwnerContentDrawer.vue", "utf8");

assert.doesNotMatch(
  drawer,
  /publicAPIURL\(`\/me\/content\/\$\{item\.id\}\/download`\)/,
  "profile copy action must not hardcode the auth-only owner download endpoint",
);
assert.ok(
  drawer.includes("basisContentDownloadPath(item)"),
  "profile copy action should use the shared Basis download path helper",
);

const helper = readFileSync("src/lib/api/download-url.ts", "utf8");

assert.ok(
  helper.includes('item.visibility === "public"'),
  "public owner assets should copy the anonymous public download endpoint",
);
assert.ok(
  helper.includes("`/content/${contentID}/download`"),
  "public owner assets should use /content/:id/download",
);
assert.ok(
  helper.includes("`/me/content/${contentID}/download`"),
  "non-public owner assets should keep the authenticated owner endpoint",
);
