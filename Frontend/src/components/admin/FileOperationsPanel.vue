<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { listAdminFiles, rescanAdminFile } from "../../lib/api/admin";
import type { AdminFile } from "../../lib/api/types";
import { BYTE_UNITS } from "../../lib/config/runtime";
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

const files = ref<AdminFile[]>([]);
const locale = props.locale ?? "en";
const t = ui[locale].adminPanels.files;
const common = ui[locale].adminPanels.common;
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
    error.value = caught instanceof Error ? caught.message : t.loadFailed;
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
    error.value = t.reasonRequired;
    showToast(error.value, "error");
    return;
  }
  actionID.value = file.id;
  error.value = "";
  try {
    const result = await rescanAdminFile(props.accessToken, file.id, rescanReason.value.trim());
    rescanReason.value = "";
    showToast(`${t.queuedPrefix} ${result.job_id}`, "success");
    await refreshFiles(true);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.queueFailed;
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
        <p class="eyebrow">{{ t.eyebrow }}</p>
        <h2 id="files-title">{{ t.title }}</h2>
      </div>
      <button class="button button--secondary" type="button" :disabled="loading || !isAuthorized" @click="refreshFiles()">{{ common.refresh }}</button>
    </div>

    <div v-if="!isAuthorized" class="message message--warning">{{ t.unauthorized }}</div>

    <div class="admin-controls">
      <label class="field">
        {{ common.search }}
        <input v-model="query" type="search" maxlength="200" :placeholder="t.searchPlaceholder" :disabled="!isAuthorized" @change="refreshFiles()" />
      </label>
      <label class="field">
        {{ t.scanStatus }}
        <select v-model="scanStatus" :disabled="!isAuthorized" @change="refreshFiles()">
          <option value="">{{ t.allScanStatuses }}</option>
          <option value="pending">{{ common.pending }}</option>
          <option value="running">{{ common.running }}</option>
          <option value="clean">{{ common.clean }}</option>
          <option value="suspicious">{{ common.suspicious }}</option>
          <option value="infected">{{ common.infected }}</option>
          <option value="failed">{{ common.failed }}</option>
          <option value="skipped">{{ common.skipped }}</option>
        </select>
      </label>
      <label class="field">
        {{ t.bucket }}
        <input v-model="bucket" type="search" maxlength="120" :disabled="!isAuthorized" @change="refreshFiles()" />
      </label>
      <label class="field">
        {{ t.contentID }}
        <input v-model="contentID" type="search" :disabled="!isAuthorized" @change="refreshFiles()" />
      </label>
      <label class="field">
        SHA-256
        <input v-model="hash" type="search" maxlength="64" :disabled="!isAuthorized" @change="refreshFiles()" />
      </label>
      <label class="field">
        {{ t.rescanReason }}
        <input v-model="rescanReason" type="text" maxlength="500" :disabled="!isAuthorized" />
      </label>
    </div>

    <div v-if="files.length === 0" class="empty-state">
      <span class="empty-state__badge">{{ t.emptyBadge }}</span>
      <div>
        <h2>{{ t.emptyTitle }}</h2>
        <p>{{ t.emptyCopy }}</p>
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
              <dt>{{ t.size }}</dt>
              <dd>{{ formatBytes(file.file_size) }}</dd>
            </div>
            <div>
              <dt>{{ common.content }}</dt>
              <dd>{{ file.content_status }}</dd>
            </div>
            <div>
              <dt>{{ t.uploader }}</dt>
              <dd>@{{ file.uploaded_by_username }}</dd>
            </div>
          </dl>
          <code>{{ file.file_hash_sha256 }}</code>
          <code>{{ compactScan(file.scan_result) }}</code>
        </div>
        <div class="moderation-actions">
          <button class="button button--secondary" type="button" :disabled="actionID === file.id || !['failed', 'suspicious', 'infected'].includes(file.scan_status)" @click="rescanFile(file)">{{ t.rescan }}</button>
        </div>
      </article>
      <button v-if="nextCursor" class="button button--secondary" type="button" :disabled="loading" @click="loadMore">
        {{ loading ? common.loading : common.loadMore }}
      </button>
    </div>
  </section>
</template>
