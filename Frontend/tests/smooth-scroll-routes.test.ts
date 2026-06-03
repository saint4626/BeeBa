import assert from "node:assert/strict";
import { shouldInitGlobalSmoothScroll } from "../src/lib/ui/smooth-scroll.ts";

const scrollReady = {
  reduceMotion: false,
  hasScrollWrapper: true,
  hasScrollContent: true,
};

for (const pathname of ["/profile", "/upload", "/admin/moderation", "/ru/profile", "/ru/upload"]) {
  assert.equal(shouldInitGlobalSmoothScroll({ ...scrollReady, pathname }), true, pathname);
}

assert.equal(
  shouldInitGlobalSmoothScroll({ ...scrollReady, pathname: "/profile", reduceMotion: true }),
  false,
);

assert.equal(
  shouldInitGlobalSmoothScroll({ ...scrollReady, pathname: "/catalog", hasScrollWrapper: false }),
  false,
);

assert.equal(
  shouldInitGlobalSmoothScroll({ ...scrollReady, pathname: "/catalog", hasScrollContent: false }),
  false,
);
