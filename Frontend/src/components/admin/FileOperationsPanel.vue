<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { listAdminFiles, rescanAdminFile } from "../../lib/api/admin";
import type { AdminFile } from "../../lib/api/types";
import { BYTE_UNITS } from "../../lib/config/runtime";
import { showToast } from "../../lib/ui/toast";

const props = defineProps<{
  accessToken: string;
  isAuthorized: boolean;
  refreshNonce?: number;
}>();

const emit = defineEmits<{
  count: [value: number];
}>();

const files = ref<AdminFile[]>([]);
const query = ref("");
const scanStatus = ref("");
const bucket = ref("");
const contentID = ref("");
const hash = ref("");
const rescanReason = ref("");
const nextCursor = ref("");
const loading = ref(false);
const actionID = ref("");
const error = ref("");

onMounted(() => {
  void refreshFiles(true);
});

watch(
  () => props.refreshNonce,
  () => {
    void refreshFiles(true);
  },
);

async function refreshFiles(silent = false) {
  nextCursor.value = "";
  files.value = [];
  await loadFiles("", silent);
}

async function loadMore() {
  await loadFiles(nextCursor.value);
}

async function loadFiles(cursor: string, silent = false) {
  if (!props.isAuthorized) return;
  loading.value = true;
  error.value = "";
  try {
    const page = await listAdminFiles(props.accessToken, {
      q: query.value.trim(),
      scan_status: scanStatus.value,
      bucket: bucket.value.trim(),
      content_id: contentID.value.trim(),
      hash: hash.value.trim(),
      cursor,
    });
    files.value = cursor ? [...files.value, ...page.items] : page.items;
    nextCursor.value = page.nextCursor;
    emit("count", files.value.length);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : "Failed to load content files.";
    emit("count", files.value.length);
    if (!silent) {
      showToast(error.value, "error");
    }
  } finally {
    loading.value = false;
  }
}

async function rescanFile(file: AdminFile) {
  if (!rescanReason.value.trim()) {
    error.value = "Reason is required for re-scan.";
    showToast(error.value, "error");
    return;
  }
  actionID.value = file.id;
  error.value = "";
  try {
    const result = await rescanAdminFile(props.accessToken, file.id, rescanReason.value.trim());
    rescanReason.value = "";
    showToast(`File queued for re-scan. Job ${result.job_id}`, "success");
    await refreshFiles(true);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : "Failed to queue re-scan.";
    showToast(error.value, "error");
  } finally {
    actionID.value = "";
  }
}

function formatBytes(value: number) {
  if (value < BYTE_UNITS.kib) return `${value} B`;
  if (value < BYTE_UNITS.mib) return `${Math.round(value / BYTE_UNITS.kib)} KB`;
  if (value < BYTE_UNITS.gib) return `${(value / BYTE_UNITS.mib).toFixed(1)} MB`;
  return `${(value / BYTE_UNITS.gib).toFixed(2)} GB`;
}

function compactScan(value: AdminFile["scan_result"]) {
  if (value === null || value === undefined) return "{}";
  const raw = JSON.stringify(value);
  return raw.length > 220 ? `${raw.slice(0, 220)}...` : raw;
}
</script>

<template>
  <section class="panel panel--wide" aria-labelledby="files-title">
    <div class="panel__head panel__head--row">
      <div>
        <p class="eyebrow">Files</p>
        <h2 id="files-title">File operations</h2>
      </div>
      <button class="button button--secondary" type="button" :disabled="loading || !isAuthorized" @click="refreshFiles()">Refresh</button>
    </div>

    <div v-if="!isAuthorized" class="message message--warning">Login with an admin or owner account to inspect content files.</div>
    <div v-if="error" class="message message--error">{{ error }}</div>

    <div class="admin-controls">
      <label class="field">
        Search
        <input v-model="query" type="search" maxlength="200" placeholder="filename or title" :disabled="!isAuthorized" @change="refreshFiles()" />
      </label>
      <label class="field">
        Scan status
        <select v-model="scanStatus" :disabled="!isAuthorized" @change="refreshFiles()">
          <option value="">All scan statuses</option>
          <option value="pending">Pending</option>
          <option value="running">Running</option>
          <option value="clean">Clean</option>
          <option value="suspicious">Suspicious</option>
          <option value="infected">Infected</option>
          <option value="failed">Failed</option>
          <option value="skipped">Skipped</option>
        </select>
      </label>
      <label class="field">
        Bucket
        <input v-model="bucket" type="search" maxlength="120" :disabled="!isAuthorized" @change="refreshFiles()" />
      </label>
      <label class="field">
        Content ID
        <input v-model="contentID" type="search" :disabled="!isAuthorized" @change="refreshFiles()" />
      </label>
      <label class="field">
        SHA-256
        <input v-model="hash" type="search" maxlength="64" :disabled="!isAuthorized" @change="refreshFiles()" />
      </label>
      <label class="field">
        Re-scan reason
        <input v-model="rescanReason" type="text" maxlength="500" :disabled="!isAuthorized" />
      </label>
    </div>

    <div v-if="files.length === 0" class="empty-state">
      <span class="empty-state__badge">No files loaded</span>
      <div>
        <h2>No content files match this filter.</h2>
        <p>Uploaded `.bee` packages appear here after they are recorded in PostgreSQL.</p>
      </div>
    </div>

    <div v-else class="moderation-list">
      <article v-for="file in files" :key="file.id" class="moderation-item moderation-item--compact">
        <div class="moderation-item__main">
          <div class="content-card__topline">
            <span class="badge" :class="{ 'badge--nsfw': ['failed', 'suspicious', 'infected'].includes(file.scan_status), 'badge--bee': file.scan_status === 'clean' }">{{ file.scan_status }}</span>
            <span class="badge">{{ file.bucket }}</span>
            <span class="badge">{{ file.content_visibility }}</span>
          </div>
          <h3>{{ file.original_filename }}</h3>
          <p>{{ file.content_title }} / @{{ file.author_username }}</p>
          <dl class="meta-grid">
            <div>
              <dt>Size</dt>
              <dd>{{ formatBytes(file.file_size) }}</dd>
            </div>
            <div>
              <dt>Content</dt>
              <dd>{{ file.content_status }}</dd>
            </div>
            <div>
              <dt>Uploader</dt>
              <dd>@{{ file.uploaded_by_username }}</dd>
            </div>
          </dl>
          <code>{{ file.file_hash_sha256 }}</code>
          <code>{{ compactScan(file.scan_result) }}</code>
        </div>
        <div class="moderation-actions">
          <button class="button button--secondary" type="button" :disabled="actionID === file.id || !['failed', 'suspicious', 'infected'].includes(file.scan_status)" @click="rescanFile(file)">Re-scan</button>
        </div>
      </article>
      <button v-if="nextCursor" class="button button--secondary" type="button" :disabled="loading" @click="loadMore">
        {{ loading ? "Loading..." : "Load more" }}
      </button>
    </div>
  </section>
</template>
