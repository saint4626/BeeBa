<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { listAuditLog } from "../../lib/api/admin";
import type { AuditLogEntry } from "../../lib/api/types";
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

const entries = ref<AuditLogEntry[]>([]);
const locale = props.locale ?? "en";
const t = ui[locale].adminPanels.audit;
const common = ui[locale].adminPanels.common;
const action = ref("");
const entityType = ref("");
const actorUserID = ref("");
const entityID = ref("");
const nextCursor = ref("");
const loading = ref(false);
const error = ref("");

onMounted(() => {
  void refreshAudit(true);
});

watch(
  () => props.refreshNonce,
  () => {
    void refreshAudit(true);
  },
);

async function refreshAudit(silent = false) {
  nextCursor.value = "";
  entries.value = [];
  await loadAudit("", silent);
}

async function loadMore() {
  await loadAudit(nextCursor.value);
}

async function loadAudit(cursor: string, silent = false) {
  if (!props.isAuthorized) return;
  loading.value = true;
  error.value = "";
  try {
    const page = await listAuditLog(props.accessToken, {
      action: action.value.trim(),
      entity_type: entityType.value.trim(),
      actor_user_id: actorUserID.value.trim(),
      entity_id: entityID.value.trim(),
      cursor,
    });
    entries.value = cursor ? [...entries.value, ...page.items] : page.items;
    nextCursor.value = page.nextCursor;
    emit("count", entries.value.length);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.failed;
    emit("count", entries.value.length);
    if (!silent) {
      showToast(error.value, "error");
    }
  } finally {
    loading.value = false;
  }
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function compactJSON(value: AuditLogEntry["after_json"]) {
  if (value === null || value === undefined) return "{}";
  const raw = JSON.stringify(value);
  return raw.length > 180 ? `${raw.slice(0, 180)}...` : raw;
}
</script>

<template>
  <section class="panel panel--wide" aria-labelledby="audit-title">
    <div class="panel__head panel__head--row">
      <div>
        <p class="eyebrow">{{ t.eyebrow }}</p>
        <h2 id="audit-title">{{ t.title }}</h2>
      </div>
      <button class="button button--secondary" type="button" :disabled="loading || !isAuthorized" @click="refreshAudit()">{{ common.refresh }}</button>
    </div>

    <div v-if="!isAuthorized" class="message message--warning">{{ t.unauthorized }}</div>

    <div class="admin-controls">
      <label class="field">
        {{ t.action }}
        <input v-model="action" type="search" maxlength="120" placeholder="content.approve" :disabled="!isAuthorized" @change="refreshAudit()" />
      </label>
      <label class="field">
        {{ t.entityType }}
        <input v-model="entityType" type="search" maxlength="80" placeholder="content" :disabled="!isAuthorized" @change="refreshAudit()" />
      </label>
      <label class="field">
        {{ t.actorUserID }}
        <input v-model="actorUserID" type="search" :disabled="!isAuthorized" @change="refreshAudit()" />
      </label>
      <label class="field">
        {{ t.entityID }}
        <input v-model="entityID" type="search" :disabled="!isAuthorized" @change="refreshAudit()" />
      </label>
    </div>

    <div v-if="entries.length === 0" class="empty-state">
      <span class="empty-state__badge">{{ t.emptyBadge }}</span>
      <div>
        <h2>{{ t.emptyTitle }}</h2>
        <p>{{ t.emptyCopy }}</p>
      </div>
    </div>

    <div v-else class="moderation-list">
      <article v-for="entry in entries" :key="entry.id" class="moderation-item moderation-item--compact">
        <div class="moderation-item__main">
          <div class="content-card__topline">
            <span class="badge badge--bee">{{ entry.action }}</span>
            <span class="badge">{{ entry.entity_type }}</span>
          </div>
          <h3>{{ entry.actor_username || entry.actor_email || "System" }}</h3>
          <p>{{ formatDate(entry.created_at) }}</p>
          <dl class="meta-grid">
            <div>
              <dt>{{ t.entity }}</dt>
              <dd>{{ entry.entity_id || common.none }}</dd>
            </div>
            <div>
              <dt>{{ t.actor }}</dt>
              <dd>{{ entry.actor_user_id || common.system }}</dd>
            </div>
            <div>
              <dt>IP</dt>
              <dd>{{ entry.ip_address || common.none }}</dd>
            </div>
          </dl>
          <code>{{ compactJSON(entry.after_json) }}</code>
        </div>
      </article>
      <button v-if="nextCursor" class="button button--secondary" type="button" :disabled="loading" @click="loadMore">
        {{ loading ? common.loading : common.loadMore }}
      </button>
    </div>
  </section>
</template>
