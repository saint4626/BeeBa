<script setup lang="ts">
import { computed, ref, watch } from "vue";
import AvatarPanel from "./AvatarPanel.vue";
import { useOwnerStore } from "../../stores/owner.store";

const owner = useOwnerStore();
const displayName = ref("");
const localError = ref("");

const publicProfileHref = computed(() => owner.user ? `/users/${owner.user.username}` : "/catalog");

watch(
  () => owner.user?.display_name,
  (value) => {
    displayName.value = value ?? "";
  },
  { immediate: true },
);

async function submitProfile() {
  localError.value = "";
  try {
    await owner.updateAccountProfile(displayName.value);
  } catch (caught) {
    localError.value = caught instanceof Error ? caught.message : "Profile update failed.";
  }
}
</script>

<template>
  <section class="panel" aria-labelledby="account-settings-title">
    <div class="panel__head panel__head--row">
      <div>
        <p class="eyebrow">Profile details</p>
        <h2 id="account-settings-title">Public identity</h2>
      </div>
      <a v-if="owner.user" class="button button--secondary" :href="publicProfileHref">Public profile</a>
    </div>

    <div class="profile-identity-layout">
      <AvatarPanel />

      <div class="profile-identity-layout__details">
        <div v-if="owner.user" class="account-summary">
          <div>
            <span>Email</span>
            <strong>{{ owner.user.email }}</strong>
          </div>
          <div>
            <span>Username</span>
            <strong>@{{ owner.user.username }}</strong>
          </div>
          <div>
            <span>Account created</span>
            <strong>{{ new Date(owner.user.created_at).toLocaleDateString() }}</strong>
          </div>
        </div>

        <form class="form-grid" @submit.prevent="submitProfile">
          <label class="field">
            Display name
            <input v-model="displayName" type="text" maxlength="80" :disabled="!owner.isAuthenticated" />
          </label>
          <button class="button button--primary" type="submit" :disabled="owner.loading || !owner.isAuthenticated">
            Save profile
          </button>
        </form>
      </div>
    </div>

    <div v-if="localError" class="message message--error">{{ localError }}</div>
  </section>
</template>
