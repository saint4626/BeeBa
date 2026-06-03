import assert from "node:assert/strict";
import { publicMediaURL } from "../src/lib/api/media-url.ts";

assert.equal(publicMediaURL("/api/v1", "avatar-id"), "/api/v1/media/avatar-id");
assert.equal(publicMediaURL("/api/v1/", "avatar-id"), "/api/v1/media/avatar-id");
assert.equal(publicMediaURL("/api/v1", "id with spaces"), "/api/v1/media/id%20with%20spaces");
assert.equal(publicMediaURL("/api/v1", ""), "");
assert.equal(publicMediaURL("/api/v1", null), "");
