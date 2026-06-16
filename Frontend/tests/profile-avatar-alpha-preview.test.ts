import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const css = readFileSync("src/styles/pages/profile.css", "utf8");
const component = readFileSync("src/components/profile/AvatarPanel.vue", "utf8");

assert.ok(
  component.includes("'avatar-preview--image': Boolean(visibleAvatarURL)"),
  "avatar preview should mark real image previews so transparent PNGs get a contrast background",
);

const imagePreviewMatch = css.match(/\.avatar-preview--image\s*\{(?<body>[^}]*)\}/);
assert.ok(imagePreviewMatch?.groups?.body, "image avatar preview needs a dedicated transparent-image background");

const body = imagePreviewMatch.groups.body;
for (const expected of [
  "background-color: var(--color-surface-raised)",
  "linear-gradient(45deg",
  "background-size: 16px 16px",
  "background-position: 0 0, 8px 8px",
]) {
  assert.ok(body.includes(expected), `avatar transparent-image preview rule should include ${expected}`);
}

assert.ok(
  component.includes("if (loading.value && !nextAvatarID)"),
  "avatar preview watcher should keep the local preview while a new avatar is processing",
);
