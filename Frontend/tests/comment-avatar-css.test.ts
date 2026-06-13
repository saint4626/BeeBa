import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const css = readFileSync("src/styles/pages/social.css", "utf8");
const component = readFileSync("src/components/content/SocialPanel.vue", "utf8");

const avatarMatch = css.match(/\.comment-avatar\s*\{(?<body>[^}]*)\}/);
assert.ok(avatarMatch?.groups?.body, "comment avatar needs an explicit sizing and centering rule");

assert.ok(
  component.includes('class="comment-item__meta"'),
  "comment author metadata should use its own class instead of broad span selectors",
);
assert.ok(css.includes(".comment-item__meta strong,"), "comment metadata text styles should target the metadata wrapper");
assert.ok(
  !css.includes(".comment-item__head strong,\n.comment-item__head span"),
  "comment header text styles must not target every span because that overrides the avatar grid centering",
);

const avatarBody = avatarMatch.groups.body;
for (const expected of [
  "width: 36px",
  "height: 36px",
  "place-items: center",
  "line-height: 1",
  "text-align: center",
  "letter-spacing: 0",
]) {
  assert.ok(avatarBody.includes(expected), `comment avatar rule should include ${expected}`);
}

assert.ok(
  /display:\s*(?:inline-)?grid/.test(avatarBody),
  "comment avatar should remain a grid formatting context so initials are centered",
);
