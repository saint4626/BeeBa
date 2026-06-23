import assert from "node:assert/strict";
import {
  buildContentCanonicalPaths,
  buildContentPath,
  contentMetaDescription,
  extractContentIDFromPathParam,
} from "../src/lib/seo/content-url.ts";
import type { PublicContentItem } from "../src/lib/api/types.ts";

const id = "11111111-1111-4111-8111-111111111111";
const item = contentItem(id);

assert.equal(buildContentPath("en", item), `/content/kubi-alt-by-bluegua-${id}`);
assert.equal(buildContentPath("ru", item), `/ru/content/kubi-alt-by-bluegua-${id}`);
assert.deepEqual(buildContentCanonicalPaths(item), {
  en: `/content/kubi-alt-by-bluegua-${id}`,
  ru: `/ru/content/kubi-alt-by-bluegua-${id}`,
  xDefault: `/content/kubi-alt-by-bluegua-${id}`,
});

assert.equal(extractContentIDFromPathParam(id), id);
assert.equal(extractContentIDFromPathParam(`kubi-alt-by-bluegua-${id}`), id);
assert.equal(extractContentIDFromPathParam(`wrong-slug-${id}`), id);
assert.equal(extractContentIDFromPathParam("not-a-content-id"), null);

const rawDescription = "Try [Basis booth](https://example.com/source) with **clean** setup.\nSource: https://example.com/raw";
assert.equal(
  contentMetaDescription({ ...item, description: rawDescription }, "en"),
  "Try Basis booth with clean setup. Source:",
);

assert.equal(
  contentMetaDescription({ ...item, description: "" }, "en"),
  "Kubi+Alt by BlueGua is a published Basis Avatars .bee asset by Momonth on BeeBa.",
);

function contentItem(contentID: string): PublicContentItem {
  return {
    id: contentID,
    slug: "kubi-alt-by-bluegua",
    title: "Kubi+Alt by BlueGua",
    description: "Public asset",
    status: "published",
    visibility: "public",
    nsfw: false,
    category: { slug: "avatars", name: "Avatars" },
    author: {
      id: "33333333-3333-4333-8333-333333333333",
      username: "momonth",
      display_name: "Momonth",
    },
    tags: [],
    likes_count: 0,
    downloads_count: 0,
    comments_count: 0,
    published_at: "2026-05-31T10:00:00.000Z",
    created_at: "2026-05-31T09:00:00.000Z",
    updated_at: "2026-06-01T10:00:00.000Z",
  };
}
