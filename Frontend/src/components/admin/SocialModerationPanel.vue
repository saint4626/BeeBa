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
import { ui, type Locale } from "../../lib/i18n";
import { showToast } from "../../lib/ui/toast";

const props = defineProps<{
  accessToken: string;
  isAuthorized: boolean;
  refreshNonce?: number;
  locale?: Locale;
}>();

const emit = defineEmits<{
  count: [value: number];
}>();

const commentStatus = ref("visible");
const locale = props.locale ?? "en";
const t = ui[locale].adminPanels.socialModeration;
const common = ui[locale].adminPanels.common;
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
    error.value = caught instanceof Error ? caught.message : t.loadFailed;
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
    showToast(t.commentApproved, "success");
    await refreshSocialQueues();
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.commentApprovalFailed;
    showToast(error.value, "error");
  } finally {
    actionID.value = "";
  }
}

async function hideCommentItem(item: ModerationCommentItem) {
  if (!props.isAuthorized) return;
  const reason = commentReason.value.trim();
  if (!reason) {
    error.value = t.hideReasonRequired;
    showToast(error.value, "error");
    return;
  }
  actionID.value = item.comment_id;
  error.value = "";
  try {
    await hideComment(props.accessToken, item.comment_id, reason);
    commentReason.value = "";
    showToast(t.commentHidden, "success");
    await refreshSocialQueues();
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.commentHideFailed;
    showToast(error.value, "error");
  } finally {
    actionID.value = "";
  }
}

async function setReportStatus(item: ModerationReportItem, status: "in_review" | "resolved" | "rejected") {
  if (!props.isAuthorized) return;
  const reason = reportReason.value.trim();
  if ((status === "resolved" || status === "rejected") && !reason) {
    error.value = t.closeReasonRequired;
    return;
  }
  actionID.value = item.report_id;
  error.value = "";
  try {
    await reviewReport(props.accessToken, item.report_id, status, reason);
    if (status !== "in_review") {
      reportReason.value = "";
    }
    showToast(`${t.reportMovedPrefix} ${status}.`, "success");
    await refreshSocialQueues();
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.reportReviewFailed;
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
        <p class="eyebrow">{{ t.eyebrow }}</p>
        <h2 id="social-queue-title">{{ t.title }}</h2>
      </div>
      <button class="button button--secondary" type="button" :disabled="loading || !isAuthorized" @click="refreshSocialQueues()">
        {{ common.refresh }}
      </button>
    </div>

    <div v-if="!isAuthorized" class="message message--warning">{{ t.unauthorized }}</div>

    <div class="admin-controls">
      <label class="field">
        {{ t.commentStatus }}
        <select v-model="commentStatus" :disabled="!isAuthorized" @change="refreshSocialQueues()">
          <option value="visible">{{ common.published }}</option>
          <option value="pending_moderation">{{ t.needsReview }}</option>
          <option value="hidden">{{ common.hidden }}</option>
          <option value="deleted">{{ common.deleted }}</option>
        </select>
      </label>
      <label class="field">
        {{ t.commentReason }}
        <input v-model="commentReason" type="text" maxlength="500" :disabled="!isAuthorized" />
      </label>
      <label class="field">
        {{ t.reportStatus }}
        <select v-model="reportStatus" :disabled="!isAuthorized" @change="refreshSocialQueues()">
          <option value="">{{ t.openInReview }}</option>
          <option value="open">{{ common.open }}</option>
          <option value="in_review">{{ common.inReview }}</option>
          <option value="resolved">{{ common.resolved }}</option>
          <option value="rejected">{{ common.rejected }}</option>
        </select>
      </label>
      <label class="field">
        {{ t.reportReason }}
        <input v-model="reportReason" type="text" maxlength="500" :disabled="!isAuthorized" />
      </label>
    </div>

    <div class="social-moderation-grid">
      <div class="moderation-column">
        <h3>{{ t.comments }}</h3>
        <div v-if="comments.length === 0" class="empty-state empty-state--compact">
          <span class="empty-state__badge">{{ t.noComments }}</span>
          <p>{{ t.noCommentsCopy }}</p>
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
              <button v-if="item.status === 'pending_moderation'" class="button button--primary" type="button" :disabled="actionID === item.comment_id" @click="approveCommentItem(item)">{{ t.approve }}</button>
              <button class="button button--secondary" type="button" :disabled="actionID === item.comment_id || !['pending_moderation', 'visible'].includes(item.status)" @click="hideCommentItem(item)">{{ t.hide }}</button>
            </div>
          </article>
        </template>
      </div>

      <div class="moderation-column">
        <h3>{{ t.reports }}</h3>
        <div v-if="reports.length === 0" class="empty-state empty-state--compact">
          <span class="empty-state__badge">{{ t.noReports }}</span>
          <p>{{ t.noReportsCopy }}</p>
        </div>
        <template v-else>
          <article v-for="item in reports" :key="item.report_id" class="moderation-item moderation-item--compact">
            <div class="moderation-item__main">
              <div class="content-card__topline">
                <span class="badge badge--bee">{{ item.status }}</span>
                <span class="badge">{{ item.reporter_display_name || item.reporter_username || t.anonymous }}</span>
              </div>
              <h3>{{ item.content_title }}</h3>
              <p>{{ item.reason }}</p>
              <small v-if="item.details">{{ item.details }}</small>
            </div>
            <div class="moderation-actions">
              <button class="button button--secondary" type="button" :disabled="actionID === item.report_id || item.status === 'in_review'" @click="setReportStatus(item, 'in_review')">{{ t.review }}</button>
              <button class="button button--primary" type="button" :disabled="actionID === item.report_id || item.status === 'resolved'" @click="setReportStatus(item, 'resolved')">{{ t.resolve }}</button>
              <button class="button button--secondary" type="button" :disabled="actionID === item.report_id || item.status === 'rejected'" @click="setReportStatus(item, 'rejected')">{{ t.reject }}</button>
            </div>
          </article>
        </template>
      </div>
    </div>
  </section>
</template>
