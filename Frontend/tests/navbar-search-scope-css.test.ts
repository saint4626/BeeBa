import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const css = readFileSync("src/styles/components/navbar.css", "utf8");
const searchRule = css.match(/\.global-search\s*\{(?<body>[^}]*)\}/);
const scopeRule = css.match(/\.global-search \.global-search__scope summary\s*\{(?<body>[^}]*)\}/);

assert.ok(searchRule?.groups?.body, "global search needs an explicit CSS rule");
assert.ok(scopeRule?.groups?.body, "global search category trigger needs a specific CSS rule");

assert.ok(
  searchRule.groups.body.includes("--global-search-scope-trigger-radius"),
  "global search needs a local radius variable for its category trigger",
);
assert.ok(
  scopeRule.groups.body.includes("border-radius: var(--global-search-scope-trigger-radius)"),
  "search category trigger must use the local radius variable",
);
assert.ok(
  scopeRule.groups.body.includes("overflow: hidden"),
  "search category trigger should clip hover background to its local radius",
);
