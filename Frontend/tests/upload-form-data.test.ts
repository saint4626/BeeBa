import assert from "node:assert/strict";
import { createContentUploadFormData } from "../src/lib/api/upload-form-data.ts";

const file = new File(["bee"], "asset.bee", { type: "application/octet-stream" });
const body = createContentUploadFormData({
  category: "avatars",
  title: "Kubi",
  description: "Avatar package",
  visibility: "public",
  unlockPassword: "basis-password",
  nsfw: false,
  file,
});

assert.equal(body.get("category"), "avatars");
assert.equal(body.get("title"), "Kubi");
assert.equal(body.get("description"), "Avatar package");
assert.equal(body.get("visibility"), "public");
assert.equal(body.get("unlock_password"), "basis-password");
assert.equal(body.get("nsfw"), "false");
assert.equal(body.get("file"), file);
assert.equal(body.get("version"), null);
