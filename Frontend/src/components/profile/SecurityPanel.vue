<script setup lang="ts">
import { computed, ref } from "vue";
import { ui, type Locale } from "../../lib/i18n";
import { showToast } from "../../lib/ui/toast";
import { useOwnerStore } from "../../stores/owner.store";

const owner = useOwnerStore();
const props = withDefaults(defineProps<{ locale?: Locale }>(), { locale: "en" });
const t = ui[props.locale].profile;
const currentPassword = ref("");
const newPassword = ref("");
const confirmPassword = ref("");

const emailStatus = computed(() => owner.user?.email_verified_at ? t.verified : t.pending);

async function submitPassword() {
  if (newPassword.value !== confirmPassword.value) {
    showToast(t.passwordMismatch, "error");
    return;
  }
  try {
    await owner.updatePassword(currentPassword.value, newPassword.value);
    currentPassword.value = "";
    newPassword.value = "";
    confirmPassword.value = "";
  } catch (caught) {
    showToast(caught instanceof Error ? caught.message : t.passwordFailed, "error");
  }
}

async function resendVerification() {
  try {
    await owner.requestEmailVerification();
  } catch (caught) {
    showToast(caught instanceof Error ? caught.message : t.verificationFailed, "error");
  }
}
</script>

<template>
  <section class="panel" aria-labelledby="security-settings-title">
    <div class="panel__head">
      <p class="eyebrow">{{ t.security }}</p>
      <h2 id="security-settings-title">{{ t.loginVerification }}</h2>
    </div>

    <div v-if="owner.user" class="account-summary account-summary--security">
      <div>
        <span>{{ t.email }}</span>
        <strong>{{ owner.user.email }}</strong>
      </div>
      <div>
        <span>{{ t.emailStatus }}</span>
        <strong>{{ emailStatus }}</strong>
      </div>
    </div>

    <button
      v-if="owner.user && !owner.user.email_verified_at"
      class="button button--secondary profile-inline-action"
      type="button"
      :disabled="owner.loading || !owner.isAuthenticated"
      @click="resendVerification"
    >
      {{ t.resendVerification }}
    </button>

    <form class="form-grid" @submit.prevent="submitPassword">
      <div class="form-row">
        <label class="field">
          {{ t.currentPassword }}
          <input v-model="currentPassword" type="password" autocomplete="current-password" minlength="1" :disabled="!owner.isAuthenticated" required />
        </label>
        <label class="field">
          {{ t.newPassword }}
          <input v-model="newPassword" type="password" autocomplete="new-password" minlength="12" maxlength="128" :disabled="!owner.isAuthenticated" required />
        </label>
      </div>
      <label class="field">
        {{ t.confirmPassword }}
        <input v-model="confirmPassword" type="password" autocomplete="new-password" minlength="12" maxlength="128" :disabled="!owner.isAuthenticated" required />
      </label>
      <button class="button button--primary" type="submit" :disabled="owner.loading || !owner.isAuthenticated">
        {{ t.changePassword }}
      </button>
    </form>
  </section>
</template>
