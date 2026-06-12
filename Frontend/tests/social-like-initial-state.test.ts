import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const socialPanel = readFileSync("src/components/content/SocialPanel.vue", "utf8");
assert.ok(socialPanel.includes("initialLiked: boolean"), "SocialPanel should accept initial liked state");
assert.ok(socialPanel.includes("const liked = ref(props.initialLiked)"), "SocialPanel liked ref should start from server state");

const enPage = readFileSync("src/pages/content/[id].astro", "utf8");
const ruPage = readFileSync("src/pages/ru/content/[id].astro", "utf8");
for (const [label, source] of [["en", enPage], ["ru", ruPage]] as const) {
  assert.ok(source.includes("initialLiked={item.liked_by_me}"), `${label} content page should pass liked_by_me into SocialPanel`);
  assert.ok(source.includes('Astro.request.headers.get("cookie")'), `${label} content page should read request cookies`);
  assert.ok(source.includes("getPublicContent(id, cookie ? { headers: { Cookie: cookie } } : undefined)"), `${label} content page should forward request cookies for viewer-aware detail`);
}

const types = readFileSync("src/lib/api/types.ts", "utf8");
assert.ok(types.includes("liked_by_me: boolean"), "PublicContentDetail type should include liked_by_me");
