import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const css = readFileSync("src/styles/components/auth-nav.css", "utf8");
const summaryMatch = css.match(/\.auth-nav__menu summary\s*\{(?<body>[^}]*)\}/);
const match = css.match(/\.auth-nav__menu summary \[data-auth-initials\]\s*\{(?<body>[^}]*)\}/);

assert.ok(summaryMatch?.groups?.body, "auth nav avatar summary needs an explicit sizing rule");
assert.ok(match?.groups?.body, "auth nav initials fallback needs an explicit centering CSS rule");

const summaryBody = summaryMatch.groups.body;
for (const expected of ["position: relative", "place-content: center", "overflow: hidden", "padding: 0", "line-height: 1"]) {
  assert.ok(summaryBody.includes(expected), `auth nav avatar summary rule should include ${expected}`);
}

const body = match.groups.body;
for (const expected of [
  "position: absolute",
  "inset: 0",
  "display: inline-flex",
  "align-items: center",
  "justify-content: center",
  "font-size: 14px",
  "font-weight: 700",
  "line-height: 1",
  "padding-top: 1px",
  "pointer-events: none",
]) {
  assert.ok(body.includes(expected), `auth nav initials fallback rule should include ${expected}`);
}
