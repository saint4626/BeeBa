import { defineStore } from "pinia";
import { computed, ref } from "vue";
import {
  approveContent,
  getAdminStatus,
  hideContent,
  listModerationQueue,
  rejectContent,
  restoreContent,
} from "../lib/api/admin";
import type { AdminStatus, ModerationQueueItem } from "../lib/api/types";
import { ADMIN_TIMING } from "../lib/config/runtime";
import { showToast } from "../lib/ui/toast";

export type AdminTabID = "content" | "social" | "users" | "files" | "jobs" | "taxonomy" | "audit";

export const ADMIN_POLL_INTERVAL_MS = ADMIN_TIMING.pollIntervalMs;

const emptyCounts: Record<AdminTabID, number | null> = {
  content: null,
  social: null,
  users: null,
  files: null,
  jobs: null,
  taxonomy: null,
  audit: null,
};

const emptyLoading: Record<AdminTabID, boolean> = {
  content: false,
  social: false,
  users: false,
  files: false,
  jobs: false,
  taxonomy: false,
  audit: false,
};

export const useAdminStore = defineStore("admin", () => {
  const status = ref<AdminStatus | null>(null);
  const ready = ref(false);
  const activeTab = ref<AdminTabID>("content");
  const tabCounts = ref<Record<AdminTabID, number | null>>({ ...emptyCounts });
  const tabLoading = ref<Record<AdminTabID, boolean>>({ ...emptyLoading });
  const lastUpdatedAt = ref<Record<AdminTabID, string>>({
    content: "",
    social: "",
    users: "",
    files: "",
    jobs: "",
    taxonomy: "",
    audit: "",
  });
  const lastError = ref("");

  const contentStatusFilter = ref("");
  const contentReason = ref("");
  const contentQueue = ref<ModerationQueueItem[]>([]);
  const contentActionID = ref("");

  const roles = computed(() => status.value?.roles ?? []);
  const actor = computed(() => status.value?.actor ?? null);
  const isAuthorized = computed(() => Boolean(status.value));
  const canModerate = computed(() => roles.value.some((role) => ["moderator", "admin", "owner"].includes(role)));
  const canAdminister = computed(() => roles.value.some((role) => ["admin", "owner"].includes(role)));

  async function bootstrap() {
    ready.value = false;
    lastError.value = "";
    try {
      status.value = await getAdminStatus("");
    } catch (caught) {
      status.value = null;
      lastError.value = caught instanceof Error ? caught.message : "Failed to confirm admin session.";
      throw caught;
    } finally {
      ready.value = true;
    }
  }

  function reset() {
    status.value = null;
    ready.value = true;
    activeTab.value = "content";
    tabCounts.value = { ...emptyCounts };
    tabLoading.value = { ...emptyLoading };
    contentStatusFilter.value = "";
    contentReason.value = "";
    contentQueue.value = [];
    contentActionID.value = "";
    lastError.value = "";
  }

  function setActiveTab(tabID: AdminTabID) {
    activeTab.value = tabID;
  }

  function setTabCount(tabID: AdminTabID, count: number) {
    tabCounts.value = {
      ...tabCounts.value,
      [tabID]: Math.max(0, count),
    };
    lastUpdatedAt.value = {
      ...lastUpdatedAt.value,
      [tabID]: new Date().toISOString(),
    };
  }

  function setTabLoading(tabID: AdminTabID, loading: boolean) {
    tabLoading.value = {
      ...tabLoading.value,
      [tabID]: loading,
    };
  }

  async function refreshContentQueue(options: { silent?: boolean } = {}) {
    if (!canModerate.value) return;
    setTabLoading("content", true);
    lastError.value = "";
    try {
      contentQueue.value = await listModerationQueue("", contentStatusFilter.value);
      setTabCount("content", contentQueue.value.length);
    } catch (caught) {
      const message = caught instanceof Error ? caught.message : "Failed to load moderation queue.";
      lastError.value = message;
      if (!options.silent) {
        showToast(message, "error");
      }
    } finally {
      setTabLoading("content", false);
    }
  }

  async function approve(item: ModerationQueueItem) {
    await actContent(item, "approve");
  }

  async function reject(item: ModerationQueueItem) {
    if (!contentReason.value.trim()) {
      showContentError("Reason is required for reject.");
      return;
    }
    await actContent(item, "reject");
  }

  async function hide(item: ModerationQueueItem) {
    if (!contentReason.value.trim()) {
      showContentError("Reason is required for hide.");
      return;
    }
    await actContent(item, "hide");
  }

  async function restore(item: ModerationQueueItem) {
    await actContent(item, "restore");
  }

  async function actContent(item: ModerationQueueItem, action: "approve" | "reject" | "hide" | "restore") {
    if (!canModerate.value) return;
    contentActionID.value = item.content_id;
    lastError.value = "";
    try {
      if (action === "approve") {
        await approveContent("", item.content_id);
        showToast("Content approved and published.", "success");
      } else if (action === "reject") {
        await rejectContent("", item.content_id, contentReason.value.trim());
        showToast("Content rejected.", "success");
      } else if (action === "hide") {
        await hideContent("", item.content_id, contentReason.value.trim());
        showToast("Content hidden.", "success");
      } else {
        await restoreContent("", item.content_id, contentReason.value.trim());
        showToast("Content restored to public catalog.", "success");
      }
      contentReason.value = "";
      await refreshContentQueue({ silent: true });
    } catch (caught) {
      showContentError(caught instanceof Error ? caught.message : "Moderation action failed.");
    } finally {
      contentActionID.value = "";
    }
  }

  function showContentError(message: string) {
    lastError.value = message;
    showToast(message, "error");
  }

  return {
    status,
    ready,
    activeTab,
    tabCounts,
    tabLoading,
    lastUpdatedAt,
    lastError,
    contentStatusFilter,
    contentReason,
    contentQueue,
    contentActionID,
    roles,
    actor,
    isAuthorized,
    canModerate,
    canAdminister,
    bootstrap,
    reset,
    setActiveTab,
    setTabCount,
    setTabLoading,
    refreshContentQueue,
    approve,
    reject,
    hide,
    restore,
  };
});
