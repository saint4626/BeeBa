<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, onMounted, reactive, watch, type PropType } from "vue";
import { storeToRefs } from "pinia";
import {
  ClipboardCheck,
  Cpu,
  HardDrive,
  MessageSquareWarning,
  ScrollText,
  Tags,
  UsersRound,
} from "lucide";
import AuditLogPanel from "./AuditLogPanel.vue";
import FileOperationsPanel from "./FileOperationsPanel.vue";
import JobOperationsPanel from "./JobOperationsPanel.vue";
import SocialModerationPanel from "./SocialModerationPanel.vue";
import TaxonomyManagementPanel from "./TaxonomyManagementPanel.vue";
import UserManagementPanel from "./UserManagementPanel.vue";
import { clearAuthSession } from "../../lib/auth/session";
import type { ModerationQueueItem } from "../../lib/api/types";
import { BYTE_UNITS } from "../../lib/config/runtime";
import { ADMIN_POLL_INTERVAL_MS, useAdminStore, type AdminTabID } from "../../stores/admin.store";

type IconNode = Array<[string, Record<string, string>]>;

interface AdminTab {
  id: AdminTabID;
  label: string;
  description: string;
  icon: IconNode;
  access: "moderation" | "admin";
}

const admin = useAdminStore();
const {
  ready,
  activeTab,
  tabLoading,
  contentStatusFilter,
  contentReason,
  contentQueue,
  contentActionID,
  lastError,
  isAuthorized,
  canModerate,
  canAdminister,
} = storeToRefs(admin);

const accessToken = "";
const loginHref = `/login?next=${encodeURIComponent("/admin/moderation")}`;
const refreshNonces = reactive<Record<AdminTabID, number>>({
  content: 0,
  social: 0,
  users: 0,
  files: 0,
  jobs: 0,
  taxonomy: 0,
  audit: 0,
});
let pollingID = 0;

const Icon = defineComponent({
  name: "AdminInlineIcon",
  props: {
    node: { type: Array as PropType<IconNode>, required: true },
    size: { type: Number, default: 15 },
  },
  setup(props) {
    return () =>
      h(
        "svg",
        {
          width: props.size,
          height: props.size,
          viewBox: "0 0 24 24",
          fill: "none",
          stroke: "currentColor",
          "stroke-width": "2",
          "stroke-linecap": "round",
          "stroke-linejoin": "round",
          "aria-hidden": "true",
          focusable: "false",
        },
        props.node.map(([tag, attrs]) => h(tag, attrs)),
      );
  },
});

const allTabs: AdminTab[] = [
  {
    id: "content",
    label: "Content",
    description: "Approve, reject, or hide uploaded packages",
    icon: ClipboardCheck as IconNode,
    access: "moderation",
  },
  {
    id: "social",
    label: "Community",
    description: "Review comments and reports",
    icon: MessageSquareWarning as IconNode,
    access: "moderation",
  },
  {
    id: "users",
    label: "Users",
    description: "Roles, bans, and account state",
    icon: UsersRound as IconNode,
    access: "admin",
  },
  {
    id: "files",
    label: "Files",
    description: "Scan state and rescan operations",
    icon: HardDrive as IconNode,
    access: "admin",
  },
  {
    id: "jobs",
    label: "Jobs",
    description: "Worker queues and retries",
    icon: Cpu as IconNode,
    access: "admin",
  },
  {
    id: "taxonomy",
    label: "Taxonomy",
    description: "Categories and tags",
    icon: Tags as IconNode,
    access: "admin",
  },
  {
    id: "audit",
    label: "Audit",
    description: "Security and moderation history",
    icon: ScrollText as IconNode,
    access: "admin",
  },
];

const visibleTabs = computed(() =>
  allTabs.filter((tab) => (tab.access === "admin" ? canAdminister.value : canModerate.value)),
);

const activeTabMeta = computed(() => visibleTabs.value.find((tab) => tab.id === activeTab.value) ?? visibleTabs.value[0]);

onMounted(async () => {
  try {
    await admin.bootstrap();
    selectInitialTab();
    await refreshActiveTab(true);
    startPolling();
  } catch {
    admin.reset();
    clearAuthSession();
    redirectToLogin();
  }
});

onBeforeUnmount(() => {
  stopPolling();
  if (typeof window !== "undefined") {
    window.removeEventListener("focus", refreshActiveTabFromFocus);
    document.removeEventListener("visibilitychange", refreshActiveTabFromVisibility);
  }
});

function redirectToLogin() {
  if (typeof window === "undefined") return;
  window.location.replace(loginHref);
}

watch(visibleTabs, (tabs) => {
  if (tabs.length === 0) return;
  if (!tabs.some((tab) => tab.id === activeTab.value)) {
    selectTab(tabs[0].id);
  }
});

watch(activeTab, () => {
  void refreshActiveTab(true);
});

function selectInitialTab() {
  const hashTab = parseHashTab();
  const fallback = visibleTabs.value[0]?.id ?? "content";
  selectTab(hashTab && visibleTabs.value.some((tab) => tab.id === hashTab) ? hashTab : fallback, false);
}

function parseHashTab(): AdminTabID | null {
  if (typeof window === "undefined") return null;
  const value = window.location.hash.replace("#", "");
  return allTabs.some((tab) => tab.id === value) ? (value as AdminTabID) : null;
}

function selectTab(tabID: AdminTabID, writeHash = true) {
  admin.setActiveTab(tabID);
  if (typeof window === "undefined" || !writeHash) return;
  const nextURL = `${window.location.pathname}${window.location.search}#${tabID}`;
  window.history.replaceState(null, "", nextURL);
}

function selectAdjacentTab(direction: 1 | -1) {
  const tabs = visibleTabs.value.map((tab) => tab.id);
  const currentIndex = tabs.indexOf(activeTab.value);
  const nextIndex = (currentIndex + direction + tabs.length) % tabs.length;
  selectTab(tabs[nextIndex]);
}

async function refreshQueue() {
  await admin.refreshContentQueue();
}

async function refreshActiveTab(silent = false) {
  if (!isAuthorized.value) return;
  if (activeTab.value === "content") {
    await admin.refreshContentQueue({ silent });
    return;
  }
  admin.setTabLoading(activeTab.value, true);
  refreshNonces[activeTab.value] += 1;
}

function startPolling() {
  if (typeof window === "undefined" || pollingID) return;
  pollingID = window.setInterval(() => {
    if (document.visibilityState !== "visible") return;
    void refreshActiveTab(true);
  }, ADMIN_POLL_INTERVAL_MS);
  window.addEventListener("focus", refreshActiveTabFromFocus);
  document.addEventListener("visibilitychange", refreshActiveTabFromVisibility);
}

function stopPolling() {
  if (!pollingID || typeof window === "undefined") return;
  window.clearInterval(pollingID);
  pollingID = 0;
}

function refreshActiveTabFromFocus() {
  void refreshActiveTab(true);
}

function refreshActiveTabFromVisibility() {
  if (document.visibilityState === "visible") {
    void refreshActiveTab(true);
  }
}

function handlePanelCount(tabID: AdminTabID, count: number) {
  admin.setTabCount(tabID, count);
  admin.setTabLoading(tabID, false);
}

function formatBytes(value?: number) {
  if (!value) return "n/a";
  if (value < BYTE_UNITS.kib) return `${value} B`;
  if (value < BYTE_UNITS.mib) return `${Math.round(value / BYTE_UNITS.kib)} KB`;
  return `${(value / BYTE_UNITS.mib).toFixed(1)} MB`;
}

function canApproveContent(item: ModerationQueueItem) {
  return ["pending_moderation", "approved"].includes(item.status) && item.scan_status === "clean";
}

function canRejectContent(item: ModerationQueueItem) {
  return ["pending_moderation", "scan_failed", "approved"].includes(item.status);
}

function canHideContent(item: ModerationQueueItem) {
  return ["published", "approved"].includes(item.status);
}

function canRestoreContent(item: ModerationQueueItem) {
  return item.status === "hidden" && item.scan_status === "clean";
}

function restoreBlockedReason(item: ModerationQueueItem) {
  if (item.status !== "hidden" || item.scan_status === "clean") return "";
  return "Clean scan required before restore.";
}

</script>

<template>
  <div class="admin-shell">
    <div v-if="!ready" class="message" aria-live="polite">Checking admin session...</div>
    <div v-if="ready && !isAuthorized" class="message message--warning">
      Login with a moderator, admin, or owner account to load moderation tools.
    </div>
    <div v-if="lastError" class="message message--error">{{ lastError }}</div>

    <section v-if="isAuthorized" class="admin-workspace" aria-labelledby="admin-workspace-title">
      <div class="admin-workspace__head">
        <div>
          <p class="eyebrow">Workspace</p>
          <h2 id="admin-workspace-title">{{ activeTabMeta?.label }}</h2>
          <p>{{ activeTabMeta?.description }}</p>
        </div>
      </div>

      <nav class="admin-tabs" role="tablist" aria-label="Admin sections">
        <button
          v-for="tab in visibleTabs"
          :key="tab.id"
          class="admin-tab"
          :class="{ 'admin-tab--active': activeTab === tab.id }"
          type="button"
          role="tab"
          :id="`admin-tab-${tab.id}`"
          :aria-selected="activeTab === tab.id"
          :aria-controls="`admin-panel-${tab.id}`"
          :tabindex="activeTab === tab.id ? 0 : -1"
          @click="selectTab(tab.id)"
          @keydown.left.prevent="selectAdjacentTab(-1)"
          @keydown.up.prevent="selectAdjacentTab(-1)"
          @keydown.right.prevent="selectAdjacentTab(1)"
          @keydown.down.prevent="selectAdjacentTab(1)"
        >
          <span class="admin-tab__label">
            <Icon :node="tab.icon" />
            <span>{{ tab.label }}</span>
          </span>
          <small>{{ tab.description }}</small>
        </button>
      </nav>

      <section
        class="admin-tab-panel"
        role="tabpanel"
        :id="`admin-panel-${activeTab}`"
        :aria-labelledby="`admin-tab-${activeTab}`"
        aria-live="polite"
      >
        <section v-if="activeTab === 'content'" class="panel panel--wide" aria-labelledby="queue-title">
          <div class="panel__head panel__head--row">
            <div>
              <p class="eyebrow">Queue</p>
              <h2 id="queue-title">Content moderation</h2>
              <p>Approve clean packages, reject invalid uploads, or hide already published content.</p>
            </div>
            <button class="button button--secondary" type="button" :disabled="tabLoading.content || !isAuthorized" @click="refreshQueue()">Refresh</button>
          </div>

          <div class="admin-controls">
            <label class="field">
              Status
              <select v-model="contentStatusFilter" :disabled="!isAuthorized" @change="refreshQueue()">
                <option value="">Pending moderation + scan failed</option>
                <option value="pending_moderation">Pending moderation</option>
                <option value="scan_failed">Scan failed</option>
                <option value="published">Published</option>
                <option value="hidden">Hidden</option>
                <option value="rejected">Rejected</option>
              </select>
            </label>
            <label class="field">
              Reason for reject/hide/restore
              <input v-model="contentReason" type="text" maxlength="500" :disabled="!isAuthorized" />
            </label>
          </div>

          <div v-if="contentQueue.length === 0" class="empty-state">
            <span class="empty-state__badge">No queue items</span>
            <div>
              <h2>No content matches this queue filter.</h2>
              <p>Valid uploads appear here after worker scan, before publication.</p>
            </div>
          </div>

          <div v-else class="moderation-list">
            <article v-for="item in contentQueue" :key="item.content_id" class="moderation-item">
              <div class="moderation-item__main">
                <div class="content-card__topline">
                  <span class="badge badge--bee">{{ item.category_name }}</span>
                  <span class="badge">{{ item.visibility }}</span>
                  <span v-if="item.nsfw" class="badge badge--nsfw">NSFW</span>
                </div>
                <h3>{{ item.title }}</h3>
                <p>{{ item.description || "No description provided." }}</p>
                <dl class="meta-grid">
                  <div>
                    <dt>Status</dt>
                    <dd>{{ item.status }}</dd>
                  </div>
                  <div>
                    <dt>Scan</dt>
                    <dd>{{ item.scan_status || "n/a" }}</dd>
                  </div>
                  <div>
                    <dt>File</dt>
                    <dd>{{ formatBytes(item.file_size) }}</dd>
                  </div>
                </dl>
                <code v-if="item.file_hash_sha256">{{ item.file_hash_sha256 }}</code>
              </div>
              <div class="moderation-actions">
                <button
                  v-if="canApproveContent(item)"
                  class="button button--primary"
                  type="button"
                  :disabled="contentActionID === item.content_id"
                  @click="admin.approve(item)"
                >
                  Approve
                </button>
                <button
                  v-if="canRejectContent(item)"
                  class="button button--secondary"
                  type="button"
                  :disabled="contentActionID === item.content_id"
                  @click="admin.reject(item)"
                >
                  Reject
                </button>
                <button
                  v-if="canHideContent(item)"
                  class="button button--secondary"
                  type="button"
                  :disabled="contentActionID === item.content_id"
                  @click="admin.hide(item)"
                >
                  Hide
                </button>
                <button
                  v-if="canRestoreContent(item)"
                  class="button button--primary"
                  type="button"
                  :disabled="contentActionID === item.content_id"
                  @click="admin.restore(item)"
                >
                  Restore
                </button>
                <button
                  v-else-if="restoreBlockedReason(item)"
                  class="button button--secondary"
                  type="button"
                  disabled
                  :title="restoreBlockedReason(item)"
                >
                  Clean scan required
                </button>
              </div>
            </article>
          </div>
        </section>

        <SocialModerationPanel
          v-else-if="activeTab === 'social'"
          :access-token="accessToken"
          :is-authorized="canModerate"
          :refresh-nonce="refreshNonces.social"
          @count="handlePanelCount('social', $event)"
        />
        <UserManagementPanel
          v-else-if="activeTab === 'users'"
          :access-token="accessToken"
          :is-authorized="canAdminister"
          :refresh-nonce="refreshNonces.users"
          @count="handlePanelCount('users', $event)"
        />
        <FileOperationsPanel
          v-else-if="activeTab === 'files'"
          :access-token="accessToken"
          :is-authorized="canAdminister"
          :refresh-nonce="refreshNonces.files"
          @count="handlePanelCount('files', $event)"
        />
        <JobOperationsPanel
          v-else-if="activeTab === 'jobs'"
          :access-token="accessToken"
          :is-authorized="canAdminister"
          :refresh-nonce="refreshNonces.jobs"
          @count="handlePanelCount('jobs', $event)"
        />
        <TaxonomyManagementPanel
          v-else-if="activeTab === 'taxonomy'"
          :access-token="accessToken"
          :is-authorized="canAdminister"
          :refresh-nonce="refreshNonces.taxonomy"
          @count="handlePanelCount('taxonomy', $event)"
        />
        <AuditLogPanel
          v-else-if="activeTab === 'audit'"
          :access-token="accessToken"
          :is-authorized="canAdminister"
          :refresh-nonce="refreshNonces.audit"
          @count="handlePanelCount('audit', $event)"
        />
      </section>
    </section>
  </div>
</template>
