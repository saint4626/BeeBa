<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import {
  approveComment,
  hideComment,
  listModerationComments,
  listModerationReports,
  reviewReport,
} from "../../lib/api/admin";
import type { ModerationCommentItem, ModerationReportItem } from "../../lib/api/types";
import { showToast } from "../../lib/ui/toast";

const props = defineProps<{
  accessToken: string;
  isAuthorized: boolean;
  refreshNonce?: number;
}>();

const emit = defineEmits<{
  count: [value: number];
}>();

const commentStatus = ref("visible");
const reportStatus = ref("");
const commentReason = ref("");
const reportReason = ref("");
const comments = ref<ModerationCommentItem[]>([]);
const reports = ref<ModerationReportItem[]>([]);
const loading = ref(false);
const actionID = ref("");
const error = ref("");

onMounted(() => {
  void refreshSocialQueues(true);
});

watch(
  () => props.refreshNonce,
  () => {
    void refreshSocialQueues(true);
  },
);

async function refreshSocialQueues(silent = false) {
  if (!props.isAuthorized) return;
  loading.value = true;
  error.value = "";
  try {
    const [commentItems, reportItems] = await Promise.all([
      listModerationComments(props.accessToken, commentStatus.value),
      listModerationReports(props.accessToken, reportStatus.value),
    ]);
    comments.value = commentItems;
    reports.value = reportItems;
    emit("count", comments.value.length + reports.value.length);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : "Failed to load social moderation queues.";
    emit("count", comments.value.length + reports.value.length);
    if (!silent) {
      showToast(error.value, "error");
    }
  } finally {
    loading.value = false;
  }
}

async function approveCommentItem(item: ModerationCommentItem) {
  if (!props.isAuthorized) return;
  actionID.value = item.comment_id;
  error.value = "";
  try {
    await approveComment(props.accessToken, item.comment_id);
    showToast("Comment approved.", "success");
    await refreshSocialQueues();
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : "Comment approval failed.";
    showToast(error.value, "error");
  } finally {
    actionID.value = "";
  }
}

async function hideCommentItem(item: ModerationCommentItem) {
  if (!props.isAuthorized) return;
  const reason = commentReason.value.trim();
  if (!reason) {
    error.value = "Reason is required for hiding comments.";
    showToast(error.value, "error");
    return;
  }
  actionID.value = item.comment_id;
  error.value = "";
  try {
    await hideComment(props.accessToken, item.comment_id, reason);
    commentReason.value = "";
    showToast("Comment hidden.", "success");
    await refreshSocialQueues();
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : "Comment hide failed.";
    showToast(error.value, "error");
  } finally {
    actionID.value = "";
  }
}

async function setReportStatus(item: ModerationReportItem, status: "in_review" | "resolved" | "rejected") {
  if (!props.isAuthorized) return;
  const reason = reportReason.value.trim();
  if ((status === "resolved" || status === "rejected") && !reason) {
    error.value = "Reason is required for closing reports.";
    return;
  }
  actionID.value = item.report_id;
  error.value = "";
  try {
    await reviewReport(props.accessToken, item.report_id, status, reason);
    if (status !== "in_review") {
      reportReason.value = "";
    }
    showToast(`Report moved to ${status}.`, "success");
    await refreshSocialQueues();
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : "Report review failed.";
    showToast(error.value, "error");
  } finally {
    actionID.value = "";
  }
}

</script>

<template>
  <section class="panel panel--wide" aria-labelledby="social-queue-title">
    <div class="panel__head panel__head--row">
      <div>
        <p class="eyebrow">Community queue</p>
        <h2 id="social-queue-title">Comments and reports</h2>
      </div>
      <button class="button button--secondary" type="button" :disabled="loading || !isAuthorized" @click="refreshSocialQueues()">
        Refresh
      </button>
    </div>

    <div v-if="!isAuthorized" class="message message--warning">Login with a moderator, admin, or owner account to review community activity.</div>

    <div class="admin-controls">
      <label class="field">
        Comment status
        <select v-model="commentStatus" :disabled="!isAuthorized" @change="refreshSocialQueues()">
          <option value="visible">Published</option>
          <option value="pending_moderation">Needs review</option>
          <option value="hidden">Hidden</option>
          <option value="deleted">Deleted</option>
        </select>
      </label>
      <label class="field">
        Comment reason
        <input v-model="commentReason" type="text" maxlength="500" :disabled="!isAuthorized" />
      </label>
      <label class="field">
        Report status
        <select v-model="reportStatus" :disabled="!isAuthorized" @change="refreshSocialQueues()">
          <option value="">Open + in review</option>
          <option value="open">Open</option>
          <option value="in_review">In review</option>
          <option value="resolved">Resolved</option>
          <option value="rejected">Rejected</option>
        </select>
      </label>
      <label class="field">
        Report reason
        <input v-model="reportReason" type="text" maxlength="500" :disabled="!isAuthorized" />
      </label>
    </div>

    <div class="social-moderation-grid">
      <div class="moderation-column">
        <h3>Comments</h3>
        <div v-if="comments.length === 0" class="empty-state empty-state--compact">
          <span class="empty-state__badge">No comments</span>
          <p>No comments match this filter.</p>
        </div>
        <template v-else>
          <article v-for="item in comments" :key="item.comment_id" class="moderation-item moderation-item--compact">
            <div class="moderation-item__main">
              <div class="content-card__topline">
                <span class="badge badge--bee">{{ item.status }}</span>
                <span class="badge">{{ item.author_display_name || item.author_username }}</span>
              </div>
              <h3>{{ item.content_title }}</h3>
              <p>{{ item.body }}</p>
            </div>
            <div class="moderation-actions">
              <button v-if="item.status === 'pending_moderation'" class="button button--primary" type="button" :disabled="actionID === item.comment_id" @click="approveCommentItem(item)">Approve</button>
              <button class="button button--secondary" type="button" :disabled="actionID === item.comment_id || !['pending_moderation', 'visible'].includes(item.status)" @click="hideCommentItem(item)">Hide</button>
            </div>
          </article>
        </template>
      </div>

      <div class="moderation-column">
        <h3>Reports</h3>
        <div v-if="reports.length === 0" class="empty-state empty-state--compact">
          <span class="empty-state__badge">No reports</span>
          <p>No reports match this filter.</p>
        </div>
        <template v-else>
          <article v-for="item in reports" :key="item.report_id" class="moderation-item moderation-item--compact">
            <div class="moderation-item__main">
              <div class="content-card__topline">
                <span class="badge badge--bee">{{ item.status }}</span>
                <span class="badge">{{ item.reporter_display_name || item.reporter_username || "anonymous" }}</span>
              </div>
              <h3>{{ item.content_title }}</h3>
              <p>{{ item.reason }}</p>
              <small v-if="item.details">{{ item.details }}</small>
            </div>
            <div class="moderation-actions">
              <button class="button button--secondary" type="button" :disabled="actionID === item.report_id || item.status === 'in_review'" @click="setReportStatus(item, 'in_review')">Review</button>
              <button class="button button--primary" type="button" :disabled="actionID === item.report_id || item.status === 'resolved'" @click="setReportStatus(item, 'resolved')">Resolve</button>
              <button class="button button--secondary" type="button" :disabled="actionID === item.report_id || item.status === 'rejected'" @click="setReportStatus(item, 'rejected')">Reject</button>
            </div>
          </article>
        </template>
      </div>
    </div>
  </section>
</template>
