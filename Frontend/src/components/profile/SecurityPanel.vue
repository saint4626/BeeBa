<script setup lang="ts">
import { computed, ref } from "vue";
import { showToast } from "../../lib/ui/toast";
import { useOwnerStore } from "../../stores/owner.store";

const owner = useOwnerStore();
const currentPassword = ref("");
const newPassword = ref("");
const confirmPassword = ref("");

const emailStatus = computed(() => owner.user?.email_verified_at ? "Verified" : "Verification pending");

async function submitPassword() {
  if (newPassword.value !== confirmPassword.value) {
    showToast("New password confirmation does not match.", "error");
    return;
  }
  try {
    await owner.updatePassword(currentPassword.value, newPassword.value);
    currentPassword.value = "";
    newPassword.value = "";
    confirmPassword.value = "";
  } catch (caught) {
    showToast(caught instanceof Error ? caught.message : "Password change failed.", "error");
  }
}

async function resendVerification() {
  try {
    await owner.requestEmailVerification();
  } catch (caught) {
    showToast(caught instanceof Error ? caught.message : "Verification request failed.", "error");
  }
}
</script>

<template>
  <section class="panel" aria-labelledby="security-settings-title">
    <div class="panel__head">
      <p class="eyebrow">Security</p>
      <h2 id="security-settings-title">Login and verification</h2>
    </div>

    <div v-if="owner.user" class="account-summary account-summary--security">
      <div>
        <span>Email</span>
        <strong>{{ owner.user.email }}</strong>
      </div>
      <div>
        <span>Email status</span>
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
      Resend verification
    </button>

    <form class="form-grid" @submit.prevent="submitPassword">
      <div class="form-row">
        <label class="field">
          Current password
          <input v-model="currentPassword" type="password" autocomplete="current-password" minlength="1" :disabled="!owner.isAuthenticated" required />
        </label>
        <label class="field">
          New password
          <input v-model="newPassword" type="password" autocomplete="new-password" minlength="12" maxlength="128" :disabled="!owner.isAuthenticated" required />
        </label>
      </div>
      <label class="field">
        Confirm new password
        <input v-model="confirmPassword" type="password" autocomplete="new-password" minlength="12" maxlength="128" :disabled="!owner.isAuthenticated" required />
      </label>
      <button class="button button--primary" type="submit" :disabled="owner.loading || !owner.isAuthenticated">
        Change password
      </button>
    </form>
  </section>
</template>
