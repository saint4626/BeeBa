<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { banAdminUser, listAdminUsers, setAdminUserRoles, unbanAdminUser } from "../../lib/api/admin";
import type { AdminUser } from "../../lib/api/types";
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

const users = ref<AdminUser[]>([]);
const locale = props.locale ?? "en";
const t = ui[locale].adminPanels.users;
const common = ui[locale].adminPanels.common;
const query = ref("");
const roleFilter = ref("");
const reason = ref("");
const loading = ref(false);
const actionID = ref("");
const error = ref("");

onMounted(() => {
  void refreshUsers(true);
});

watch(
  () => props.refreshNonce,
  () => {
    void refreshUsers(true);
  },
);

async function refreshUsers(silent = false) {
  if (!props.isAuthorized) return;
  loading.value = true;
  error.value = "";
  try {
    users.value = await listAdminUsers(props.accessToken, query.value.trim(), roleFilter.value);
    emit("count", users.value.length);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.loadFailed;
    emit("count", users.value.length);
    if (!silent) {
      showToast(error.value, "error");
    }
  } finally {
    loading.value = false;
  }
}

async function banUser(user: AdminUser) {
  if (!reason.value.trim()) {
    error.value = t.reasonRequired;
    showToast(error.value, "error");
    return;
  }
  await updateUser(user, "ban");
}

async function unbanUser(user: AdminUser) {
  await updateUser(user, "unban");
}

async function toggleRole(user: AdminUser, role: "moderator" | "admin") {
  if (!props.isAuthorized) return;
  actionID.value = user.id;
  error.value = "";
  try {
    const roles = new Set(user.roles);
    if (roles.has(role)) {
      roles.delete(role);
    } else {
      roles.add(role);
    }
    roles.add("user");
    await setAdminUserRoles(props.accessToken, user.id, Array.from(roles));
    showToast(t.rolesUpdated, "success");
    await refreshUsers(true);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.rolesUpdateFailed;
    showToast(error.value, "error");
  } finally {
    actionID.value = "";
  }
}

async function updateUser(user: AdminUser, action: "ban" | "unban") {
  if (!props.isAuthorized) return;
  actionID.value = user.id;
  error.value = "";
  try {
    if (action === "ban") {
      await banAdminUser(props.accessToken, user.id, reason.value.trim());
    } else {
      await unbanAdminUser(props.accessToken, user.id, reason.value.trim());
    }
    reason.value = "";
    showToast(action === "ban" ? t.userBanned : t.userUnbanned, "success");
    await refreshUsers(true);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.userUpdateFailed;
    showToast(error.value, "error");
  } finally {
    actionID.value = "";
  }
}
</script>

<template>
  <section class="panel panel--wide" aria-labelledby="users-title">
    <div class="panel__head panel__head--row">
      <div>
        <p class="eyebrow">{{ t.eyebrow }}</p>
        <h2 id="users-title">{{ t.title }}</h2>
      </div>
      <button class="button button--secondary" type="button" :disabled="loading || !isAuthorized" @click="refreshUsers()">{{ common.refresh }}</button>
    </div>

    <div v-if="!isAuthorized" class="message message--warning">{{ t.unauthorized }}</div>

    <div class="admin-controls">
      <label class="field">
        {{ common.search }}
        <input v-model="query" type="search" maxlength="200" :disabled="!isAuthorized" @change="refreshUsers()" />
      </label>
      <label class="field">
        {{ t.role }}
        <select v-model="roleFilter" :disabled="!isAuthorized" @change="refreshUsers()">
          <option value="">{{ t.allRoles }}</option>
          <option value="user">{{ common.user }}</option>
          <option value="moderator">{{ common.moderator }}</option>
          <option value="admin">{{ common.admin }}</option>
          <option value="owner">{{ common.owner }}</option>
        </select>
      </label>
      <label class="field">
        {{ t.banReason }}
        <input v-model="reason" type="text" maxlength="500" :disabled="!isAuthorized" />
      </label>
    </div>

    <div v-if="users.length === 0" class="empty-state">
      <span class="empty-state__badge">{{ t.emptyBadge }}</span>
      <div>
        <h2>{{ t.emptyTitle }}</h2>
        <p>{{ t.emptyCopy }}</p>
      </div>
    </div>

    <div v-else class="moderation-list">
      <article v-for="user in users" :key="user.id" class="moderation-item moderation-item--compact">
        <div class="moderation-item__main">
          <div class="content-card__topline">
            <span class="badge" :class="{ 'badge--nsfw': user.banned_at }">{{ user.banned_at ? t.banned : common.active }}</span>
            <span v-for="role in user.roles" :key="role" class="badge badge--bee">{{ role }}</span>
          </div>
          <h3>{{ user.display_name || user.username }}</h3>
          <p>{{ user.email }} / @{{ user.username }}</p>
          <dl class="meta-grid">
            <div>
              <dt>{{ common.content }}</dt>
              <dd>{{ user.content_count }}</dd>
            </div>
            <div>
              <dt>{{ common.email }}</dt>
              <dd>{{ user.email_verified_at ? common.verified : common.pending }}</dd>
            </div>
            <div>
              <dt>{{ t.userID }}</dt>
              <dd>{{ user.id }}</dd>
            </div>
          </dl>
        </div>
        <div class="moderation-actions">
          <button class="button button--secondary" type="button" :disabled="actionID === user.id || user.roles.includes('owner')" @click="toggleRole(user, 'moderator')">
            {{ user.roles.includes("moderator") ? t.removeModerator : t.makeModerator }}
          </button>
          <button class="button button--secondary" type="button" :disabled="actionID === user.id || user.roles.includes('owner')" @click="toggleRole(user, 'admin')">
            {{ user.roles.includes("admin") ? t.removeAdmin : t.makeAdmin }}
          </button>
          <button v-if="!user.banned_at" class="button button--secondary" type="button" :disabled="actionID === user.id || user.roles.includes('owner')" @click="banUser(user)">{{ t.ban }}</button>
          <button v-else class="button button--secondary" type="button" :disabled="actionID === user.id" @click="unbanUser(user)">{{ t.unban }}</button>
        </div>
      </article>
    </div>
  </section>
</template>
