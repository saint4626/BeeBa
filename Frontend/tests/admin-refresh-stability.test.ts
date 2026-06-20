import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const panels = [
  {
    file: "src/components/admin/AuditLogPanel.vue",
    refreshFunction: "refreshAudit",
    stateName: "entries",
  },
  {
    file: "src/components/admin/FileOperationsPanel.vue",
    refreshFunction: "refreshFiles",
    stateName: "files",
  },
  {
    file: "src/components/admin/JobOperationsPanel.vue",
    refreshFunction: "refreshJobs",
    stateName: "jobs",
  },
];

for (const panel of panels) {
  const source = readFileSync(panel.file, "utf8");
  const refreshMatch = source.match(
    new RegExp(`async function ${panel.refreshFunction}\\([^)]*\\) \\{(?<body>[\\s\\S]*?)\\n\\}`),
  );

  assert.ok(refreshMatch?.groups?.body, `${panel.refreshFunction} should stay inspectable`);
  assert.ok(
    !refreshMatch.groups.body.includes(`${panel.stateName}.value = []`),
    `${panel.refreshFunction} must keep existing ${panel.stateName} visible while refetching`,
  );
}

const moderationApp = readFileSync("src/components/admin/ModerationApp.vue", "utf8");
const activeRefreshMatch = moderationApp.match(/async function refreshActiveTab\([^)]*\) \{(?<body>[\s\S]*?)\n\}/);

assert.ok(activeRefreshMatch?.groups?.body, "refreshActiveTab should stay inspectable");
assert.ok(
  activeRefreshMatch.groups.body.includes("tabLoading.value["),
  "active admin refresh should skip duplicate requests while the current tab is already loading",
);
