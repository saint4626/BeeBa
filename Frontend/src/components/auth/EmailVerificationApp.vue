<script setup lang="ts">
import { onMounted, ref } from "vue";
import { verifyEmail } from "../../lib/api/auth";
import { ui, type Locale } from "../../lib/i18n";
import type { PublicUser } from "../../lib/api/types";
import { publishSessionUser, readAuthSession } from "../../lib/auth/session";
import { showToast } from "../../lib/ui/toast";
import { navigateWithPageProgress } from "../../lib/ui/page-progress";

const props = withDefaults(defineProps<{ locale?: Locale; profileHref: string }>(), { locale: "en" });
const t = ui[props.locale].verifyEmail;
const homeHref = props.locale === "ru" ? "/ru" : "/";
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
    if (readAuthSession()) {
      publishSessionUser(user.value);
    }
    showToast(`${t.successPrefix} ${user.value.email}.`, "success");
    navigateWithPageProgress(props.profileHref, "replace");
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.failed;
    showToast(error.value, "error");
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
  <section class="auth-page auth-page--verification" aria-labelledby="verify-email-title">
    <div class="auth-card auth-card--verification">
      <a class="auth-card__brand" :href="homeHref" aria-label="BeeBa">
        <img src="/brand/beeba-logo-256.webp" alt="" width="48" height="48" decoding="async" />
      </a>
      <p class="eyebrow">{{ t.eyebrow }}</p>
      <h1 id="verify-email-title">{{ t.heading }}</h1>

      <form class="auth-form" @submit.prevent="submit">
        <label class="auth-field">
          <span>{{ t.token }}</span>
          <input v-model="token" type="text" autocomplete="one-time-code" required />
        </label>
        <button class="auth-submit" type="submit" :disabled="loading">
          {{ loading ? t.checking : t.action }}
        </button>
      </form>

      <div v-if="user" class="message message--success">{{ t.successPrefix }} {{ user.email }}.</div>
      <div v-if="error" class="message message--error">{{ error }}</div>
    </div>
  </section>
</template>
