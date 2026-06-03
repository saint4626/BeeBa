import assert from "node:assert/strict";
import {
  shouldStartPageProgressForForm,
  shouldStartPageProgressForLink,
} from "../src/lib/ui/page-progress.ts";

const currentURL = "https://beeba.example/catalog";

assert.equal(
  shouldStartPageProgressForLink({ currentURL, href: "/profile" }),
  true,
  "normal same-tab links should start progress",
);

assert.equal(
  shouldStartPageProgressForLink({ currentURL, href: "https://status.example" }),
  true,
  "same-tab external links should start progress before the document unloads",
);

assert.equal(
  shouldStartPageProgressForLink({ currentURL, href: "/catalog#top" }),
  false,
  "same-page hash links should not start page progress",
);

assert.equal(
  shouldStartPageProgressForLink({ currentURL, href: "/upload", ariaDisabled: true }),
  false,
  "disabled links should not start progress",
);

assert.equal(
  shouldStartPageProgressForLink({ currentURL, href: "/api/v1/content/1/download", download: true }),
  false,
  "downloads should not start page progress",
);

assert.equal(
  shouldStartPageProgressForLink({ currentURL, href: "mailto:hello@example.com" }),
  false,
  "mailto links should not start page progress",
);

assert.equal(
  shouldStartPageProgressForLink({ currentURL, href: "/catalog", hasModifierKey: true }),
  false,
  "modified clicks should not start same-tab progress",
);

assert.equal(
  shouldStartPageProgressForForm({ defaultPrevented: false, target: "" }),
  true,
  "normal form submits should start progress",
);

assert.equal(
  shouldStartPageProgressForForm({ defaultPrevented: true, target: "" }),
  false,
  "AJAX/prevented forms should not start page progress",
);

assert.equal(
  shouldStartPageProgressForForm({ defaultPrevented: false, target: "_blank" }),
  false,
  "new-tab forms should not start same-tab progress",
);
