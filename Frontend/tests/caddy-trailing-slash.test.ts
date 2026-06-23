import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const caddyfiles = [
  "../infra/caddy/Caddyfile.dev",
  "../infra/caddy/Caddyfile.prod",
];

for (const filePath of caddyfiles) {
  const source = readFileSync(filePath, "utf8");
  assert.match(source, /@canonicalTrailingSlash\s*\{[\s\S]*path_regexp\s+canonicalTrailingSlash\s+\^\(\.\+\)\/\+\$/);
  assert.match(source, /not\s+path\s+\/api\/\*\s+\/healthz\s+\/readyz\s+\/openapi\.yaml/);
  assert.match(source, /redir\s+@canonicalTrailingSlash\s+\{re\.canonicalTrailingSlash\.1\}\s+301/);
  assert.ok(
    source.indexOf("redir @canonicalTrailingSlash") < source.indexOf("reverse_proxy frontend:4321"),
    `${filePath} must redirect canonical trailing slash before proxying frontend requests`,
  );
}
