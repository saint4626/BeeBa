<script setup lang="ts">
import { onMounted, ref } from "vue";
import { verifyEmail } from "../../lib/api/auth";
import type { PublicUser } from "../../lib/api/types";

const token = ref("");
const user = ref<PublicUser | null>(null);
const loading = ref(false);
const error = ref("");

async function submit() {
  loading.value = true;
  error.value = "";
  user.value = null;
  try {
    user.value = await verifyEmail(token.value.trim());
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : "Email verification failed.";
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  const url = new URL(window.location.href);
  token.value = url.searchParams.get("token") || "";
  if (token.value) {
    void submit();
  }
});
</script>

<template>
  <section class="panel" aria-labelledby="verify-email-title">
    <div class="panel__head">
      <p class="eyebrow">Account</p>
      <h2 id="verify-email-title">Email verification</h2>
    </div>

    <form class="form-grid" @submit.prevent="submit">
      <label class="field">
        Verification token
        <input v-model="token" type="text" autocomplete="one-time-code" required />
      </label>
      <button class="button button--primary" type="submit" :disabled="loading">
        {{ loading ? "Checking..." : "Verify email" }}
      </button>
    </form>

    <div v-if="user" class="message message--success">Email verified for {{ user.email }}.</div>
    <div v-if="error" class="message message--error">{{ error }}</div>
  </section>
</template>
