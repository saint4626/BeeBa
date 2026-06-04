<script setup lang="ts">
import { onMounted, ref } from "vue";
import { verifyEmail } from "../../lib/api/auth";
import { ui, type Locale } from "../../lib/i18n";
import type { PublicUser } from "../../lib/api/types";

const props = withDefaults(defineProps<{ locale?: Locale }>(), { locale: "en" });
const t = ui[props.locale].verifyEmail;
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
    error.value = caught instanceof Error ? caught.message : t.failed;
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
      <p class="eyebrow">{{ t.eyebrow }}</p>
      <h2 id="verify-email-title">{{ t.heading }}</h2>
    </div>

    <form class="form-grid" @submit.prevent="submit">
      <label class="field">
        {{ t.token }}
        <input v-model="token" type="text" autocomplete="one-time-code" required />
      </label>
      <button class="button button--primary" type="submit" :disabled="loading">
        {{ loading ? t.checking : t.action }}
      </button>
    </form>

    <div v-if="user" class="message message--success">{{ t.successPrefix }} {{ user.email }}.</div>
    <div v-if="error" class="message message--error">{{ error }}</div>
  </section>
</template>
