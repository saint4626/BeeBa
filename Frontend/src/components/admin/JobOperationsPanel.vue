<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { listAdminJobs, retryAdminJob } from "../../lib/api/admin";
import type { AdminJob } from "../../lib/api/types";
import { showToast } from "../../lib/ui/toast";

const props = defineProps<{
  accessToken: string;
  isAuthorized: boolean;
  refreshNonce?: number;
}>();

const emit = defineEmits<{
  count: [value: number];
}>();

const jobs = ref<AdminJob[]>([]);
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
  jobs.value = [];
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
    error.value = caught instanceof Error ? caught.message : "Failed to load worker jobs.";
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
    error.value = "Reason is required for retry.";
    showToast(error.value, "error");
    return;
  }
  actionID.value = job.id;
  error.value = "";
  try {
    await retryAdminJob(props.accessToken, job.id, retryReason.value.trim());
    retryReason.value = "";
    showToast("Job queued for retry.", "success");
    await refreshJobs(true);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : "Failed to retry job.";
    showToast(error.value, "error");
  } finally {
    actionID.value = "";
  }
}

function formatDate(value?: string | null) {
  if (!value) return "n/a";
  return new Intl.DateTimeFormat("en", {
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
        <p class="eyebrow">Workers</p>
        <h2 id="jobs-title">Job operations</h2>
      </div>
      <button class="button button--secondary" type="button" :disabled="loading || !isAuthorized" @click="refreshJobs()">Refresh</button>
    </div>

    <div v-if="!isAuthorized" class="message message--warning">Login with an admin or owner account to inspect worker jobs.</div>

    <div class="admin-controls">
      <label class="field">
        Queue
        <select v-model="queue" :disabled="!isAuthorized" @change="refreshJobs()">
          <option value="">All queues</option>
          <option value="file_scan_queue">File scan</option>
          <option value="image_processing_queue">Image processing</option>
          <option value="search_index_queue">Search index</option>
          <option value="email_queue">Email</option>
          <option value="moderation_queue">Moderation</option>
          <option value="cleanup_queue">Cleanup</option>
        </select>
      </label>
      <label class="field">
        Status
        <select v-model="status" :disabled="!isAuthorized" @change="refreshJobs()">
          <option value="">All statuses</option>
          <option value="pending">Pending</option>
          <option value="running">Running</option>
          <option value="succeeded">Succeeded</option>
          <option value="failed">Failed</option>
          <option value="dead">Dead</option>
        </select>
      </label>
      <label class="field">
        Job type
        <input v-model="jobType" type="search" maxlength="120" placeholder="scan_content_file" :disabled="!isAuthorized" @change="refreshJobs()" />
      </label>
      <label class="field">
        Retry reason
        <input v-model="retryReason" type="text" maxlength="500" :disabled="!isAuthorized" />
      </label>
    </div>

    <div v-if="jobs.length === 0" class="empty-state">
      <span class="empty-state__badge">No jobs loaded</span>
      <div>
        <h2>No worker jobs match this filter.</h2>
        <p>Scan, image, and search jobs appear here after uploads and moderation actions.</p>
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
          <p>{{ job.last_error || "No worker error recorded." }}</p>
          <dl class="meta-grid">
            <div>
              <dt>Attempts</dt>
              <dd>{{ job.attempt_count }} / {{ job.max_attempts }}</dd>
            </div>
            <div>
              <dt>Next retry</dt>
              <dd>{{ formatDate(job.next_retry_at) }}</dd>
            </div>
            <div>
              <dt>Locked by</dt>
              <dd>{{ job.locked_by || "n/a" }}</dd>
            </div>
          </dl>
          <code>{{ compactPayload(job.payload) }}</code>
        </div>
        <div class="moderation-actions">
          <button class="button button--secondary" type="button" :disabled="actionID === job.id || !['failed', 'dead'].includes(job.status)" @click="retryJob(job)">Retry</button>
        </div>
      </article>
      <button v-if="nextCursor" class="button button--secondary" type="button" :disabled="loading" @click="loadMore">
        {{ loading ? "Loading..." : "Load more" }}
      </button>
    </div>
  </section>
</template>
