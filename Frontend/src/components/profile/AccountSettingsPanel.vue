<script setup lang="ts">
import { ref, watch } from "vue";
import AvatarPanel from "./AvatarPanel.vue";
import { ui, type Locale } from "../../lib/i18n";
import { showToast } from "../../lib/ui/toast";
import { useOwnerStore } from "../../stores/owner.store";

const owner = useOwnerStore();
const props = withDefaults(defineProps<{ locale?: Locale }>(), { locale: "en" });
const locale = props.locale;
const t = ui[locale].profile;
const displayName = ref("");

watch(
  () => owner.user?.display_name,
  (value) => {
    displayName.value = value ?? "";
  },
  { immediate: true },
);

async function submitProfile() {
  try {
    await owner.updateAccountProfile(displayName.value);
  } catch (caught) {
    showToast(caught instanceof Error ? caught.message : t.updateFailed, "error");
  }
}
</script>

<template>
  <section class="panel" aria-labelledby="account-settings-title">
    <div class="panel__head panel__head--row">
      <div>
        <p class="eyebrow">{{ t.profileDetails }}</p>
        <h2 id="account-settings-title">{{ t.publicIdentity }}</h2>
      </div>
    </div>

    <div class="profile-identity-layout">
      <AvatarPanel :locale="locale" />

      <div class="profile-identity-layout__details">
        <div v-if="owner.user" class="account-summary">
          <div>
            <span>{{ t.email }}</span>
            <strong>{{ owner.user.email }}</strong>
          </div>
          <div>
            <span>{{ t.username }}</span>
            <strong>@{{ owner.user.username }}</strong>
          </div>
          <div>
            <span>{{ t.accountCreated }}</span>
            <strong>{{ new Date(owner.user.created_at).toLocaleDateString() }}</strong>
          </div>
        </div>

        <form class="form-grid" @submit.prevent="submitProfile">
          <label class="field">
            {{ t.displayName }}
            <input v-model="displayName" type="text" maxlength="80" :disabled="!owner.isAuthenticated" />
          </label>
          <button class="button button--primary" type="submit" :disabled="owner.loading || !owner.isAuthenticated">
            {{ t.saveProfile }}
          </button>
        </form>
      </div>
    </div>
  </section>
</template>
