import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const uploadsAPI = readFileSync("src/lib/api/uploads.ts", "utf8");
const ownerStore = readFileSync("src/stores/owner.store.ts", "utf8");
const ownerPanel = readFileSync("src/components/profile/OwnerContentPanel.vue", "utf8");
const apiTypes = readFileSync("src/lib/api/types.ts", "utf8");

assert.ok(
  uploadsAPI.includes("next_cursor") && uploadsAPI.includes("cursor"),
  "owner content API client must follow cursor pagination instead of returning only the first page",
);
assert.ok(
  /while\s*\(|do\s*\{/.test(uploadsAPI),
  "owner content API client should keep requesting pages while next_cursor is present",
);
assert.ok(
  apiTypes.includes("OwnerStorageUsage") && apiTypes.includes("used_bytes") && apiTypes.includes("limit_bytes"),
  "typed owner content response should expose storage usage",
);
assert.ok(
  ownerStore.includes("storageUsage") && ownerStore.includes("response.storage"),
  "owner store should retain storage usage from /me/content",
);
assert.ok(
  ownerPanel.includes("<progress") && ownerPanel.includes("storage-meter"),
  "profile content panel should display account storage usage with a native progress element",
);
