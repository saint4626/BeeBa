<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { listAdminJobs, retryAdminJob } from "../../lib/api/admin";
import type { AdminJob } from "../../lib/api/types";
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

const jobs = ref<AdminJob[]>([]);
const locale = props.locale ?? "en";
const t = ui[locale].adminPanels.jobs;
const common = ui[locale].adminPanels.common;
const queue = ref("");
const status = ref("");
const jobType = ref("");
const retryReason = ref("");
const nextCursor = ref("");
const loading = ref(false);
const actionID = ref("");
const error = ref("");

onMounted(() => {
  void refreshJobs(true);
});

watch(
  () => props.refreshNonce,
  () => {
    void refreshJobs(true);
  },
);

async function refreshJobs(silent = false) {
  nextCursor.value = "";
  await loadJobs("", silent);
}

async function loadMore() {
  await loadJobs(nextCursor.value);
}

async function loadJobs(cursor: string, silent = false) {
  if (!props.isAuthorized) return;
  loading.value = true;
  error.value = "";
  try {
    const page = await listAdminJobs(props.accessToken, {
      queue: queue.value,
      status: status.value,
      job_type: jobType.value.trim(),
      cursor,
    });
    jobs.value = cursor ? [...jobs.value, ...page.items] : page.items;
    nextCursor.value = page.nextCursor;
    emit("count", jobs.value.length);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.loadFailed;
    emit("count", jobs.value.length);
    if (!silent) {
      showToast(error.value, "error");
    }
  } finally {
    loading.value = false;
  }
}

async function retryJob(job: AdminJob) {
  if (!retryReason.value.trim()) {
    error.value = t.reasonRequired;
    showToast(error.value, "error");
    return;
  }
  actionID.value = job.id;
  error.value = "";
  try {
    await retryAdminJob(props.accessToken, job.id, retryReason.value.trim());
    retryReason.value = "";
    showToast(t.retryQueued, "success");
    await refreshJobs(true);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.retryFailed;
    showToast(error.value, "error");
  } finally {
    actionID.value = "";
  }
}

function formatDate(value?: string | null) {
  if (!value) return common.none;
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function compactPayload(value: AdminJob["payload"]) {
  if (value === null || value === undefined) return "{}";
  const raw = JSON.stringify(value);
  return raw.length > 220 ? `${raw.slice(0, 220)}...` : raw;
}
</script>

<template>
  <section class="panel panel--wide" aria-labelledby="jobs-title">
    <div class="panel__head panel__head--row">
      <div>
        <p class="eyebrow">{{ t.eyebrow }}</p>
        <h2 id="jobs-title">{{ t.title }}</h2>
      </div>
      <button class="button button--secondary" type="button" :disabled="loading || !isAuthorized" @click="refreshJobs()">{{ common.refresh }}</button>
    </div>

    <div v-if="!isAuthorized" class="message message--warning">{{ t.unauthorized }}</div>

    <div class="admin-controls">
      <label class="field">
        {{ t.queue }}
        <select v-model="queue" :disabled="!isAuthorized" @change="refreshJobs()">
          <option value="">{{ t.allQueues }}</option>
          <option value="file_scan_queue">{{ t.fileScan }}</option>
          <option value="image_processing_queue">{{ t.imageProcessing }}</option>
          <option value="search_index_queue">{{ t.searchIndex }}</option>
          <option value="server_check_queue">{{ t.serverCheck }}</option>
          <option value="email_queue">{{ common.email }}</option>
          <option value="moderation_queue">{{ t.moderation }}</option>
          <option value="cleanup_queue">{{ t.cleanup }}</option>
        </select>
      </label>
      <label class="field">
        {{ common.status }}
        <select v-model="status" :disabled="!isAuthorized" @change="refreshJobs()">
          <option value="">{{ t.allStatuses }}</option>
          <option value="pending">{{ common.pending }}</option>
          <option value="running">{{ common.running }}</option>
          <option value="succeeded">{{ t.succeeded }}</option>
          <option value="failed">{{ common.failed }}</option>
          <option value="dead">{{ t.dead }}</option>
        </select>
      </label>
      <label class="field">
        {{ t.jobType }}
        <input v-model="jobType" type="search" maxlength="120" placeholder="scan_content_file" :disabled="!isAuthorized" @change="refreshJobs()" />
      </label>
      <label class="field">
        {{ t.retryReason }}
        <input v-model="retryReason" type="text" maxlength="500" :disabled="!isAuthorized" />
      </label>
    </div>

    <div v-if="jobs.length === 0" class="empty-state">
      <span class="empty-state__badge">{{ t.emptyBadge }}</span>
      <div>
        <h2>{{ t.emptyTitle }}</h2>
        <p>{{ t.emptyCopy }}</p>
      </div>
    </div>

    <div v-else class="moderation-list">
      <article v-for="job in jobs" :key="job.id" class="moderation-item moderation-item--compact">
        <div class="moderation-item__main">
          <div class="content-card__topline">
            <span class="badge" :class="{ 'badge--nsfw': ['failed', 'dead'].includes(job.status), 'badge--bee': job.status === 'pending' }">{{ job.status }}</span>
            <span class="badge">{{ job.queue_name }}</span>
          </div>
          <h3>{{ job.job_type }}</h3>
          <p>{{ job.last_error || t.noError }}</p>
          <dl class="meta-grid">
            <div>
              <dt>{{ t.attempts }}</dt>
              <dd>{{ job.attempt_count }} / {{ job.max_attempts }}</dd>
            </div>
            <div>
              <dt>{{ t.nextRetry }}</dt>
              <dd>{{ formatDate(job.next_retry_at) }}</dd>
            </div>
            <div>
              <dt>{{ t.lockedBy }}</dt>
              <dd>{{ job.locked_by || common.none }}</dd>
            </div>
          </dl>
          <code>{{ compactPayload(job.payload) }}</code>
        </div>
        <div class="moderation-actions">
          <button class="button button--secondary" type="button" :disabled="actionID === job.id || !['failed', 'dead'].includes(job.status)" @click="retryJob(job)">{{ t.retry }}</button>
        </div>
      </article>
      <button v-if="nextCursor" class="button button--secondary" type="button" :disabled="loading" @click="loadMore">
        {{ loading ? common.loading : common.loadMore }}
      </button>
    </div>
  </section>
</template>
