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
assert.ok(
  drawer.includes("getOwnedContentDownloadLink"),
  "private owner assets should request an unlisted download link instead of copying an authenticated /me URL",
);
assert.ok(
  drawer.includes('item.visibility === "private"'),
  "profile copy action should branch private assets to the unlisted link endpoint",
);

const helper = readFileSync("src/lib/api/download-url.ts", "utf8");

assert.ok(
  helper.includes("`/content/${contentID}/download`"),
  "public owner assets should use /content/:id/download",
);
assert.doesNotMatch(
  helper,
  /\/me\/content/,
  "Basis copy links must not use authenticated /me download endpoints",
);
