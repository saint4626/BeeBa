import assert from "node:assert/strict";
import { getPostLogoutRedirectHref } from "../src/lib/auth/logout-redirect.ts";

assert.equal(
  getPostLogoutRedirectHref(new URL("https://beeba.example/admin/moderation#jobs")),
  "/login?next=%2Fadmin%2Fmoderation%23jobs",
);

assert.equal(
  getPostLogoutRedirectHref(new URL("https://beeba.example/profile?tab=security")),
  "/login?next=%2Fprofile%3Ftab%3Dsecurity",
);

assert.equal(
  getPostLogoutRedirectHref(new URL("https://beeba.example/ru/upload")),
  "/ru/login?next=%2Fru%2Fupload",
);

assert.equal(
  getPostLogoutRedirectHref(new URL("https://beeba.example/catalog/worlds")),
  null,
);
