<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { confirmEmailChange, confirmPasswordChange } from "../../lib/api/auth";
import type { PublicUser } from "../../lib/api/types";
import { clearAuthSession, publishSessionUser, readAuthSession } from "../../lib/auth/session";
import { ui, type Locale } from "../../lib/i18n";
import { showToast } from "../../lib/ui/toast";
import { navigateWithPageProgress } from "../../lib/ui/page-progress";

const props = defineProps<{
  kind: "password" | "email";
  locale?: Locale;
  profileHref: string;
  loginHref: string;
}>();

const locale = props.locale ?? "en";
const t = ui[locale].accountConfirm;
const homeHref = locale === "ru" ? "/ru" : "/";
const token = ref("");
const user = ref<PublicUser | null>(null);
const loading = ref(false);
const error = ref("");
const success = ref("");

const copy = computed(() => {
  if (props.kind === "password") {
    return {
      eyebrow: t.passwordEyebrow,
      heading: t.passwordHeading,
      token: t.passwordToken,
      checking: t.passwordChecking,
      action: t.passwordAction,
      success: t.passwordSuccess,
      failed: t.passwordFailed,
      detail: t.passwordSessionsRevoked,
    };
  }
  return {
    eyebrow: t.emailEyebrow,
    heading: t.emailHeading,
    token: t.emailToken,
    checking: t.emailChecking,
    action: t.emailAction,
    success: t.emailSuccessPrefix,
    failed: t.emailFailed,
    detail: "",
  };
});

async function submit() {
  loading.value = true;
  error.value = "";
  success.value = "";
  user.value = null;
  try {
    const confirmed = props.kind === "password"
      ? await confirmPasswordChange(token.value.trim())
      : await confirmEmailChange(token.value.trim());
    user.value = confirmed;
    if (props.kind === "password") {
      clearAuthSession();
      success.value = copy.value.success;
      showToast(success.value, "success");
      navigateWithPageProgress(props.loginHref, "replace");
    } else {
      if (readAuthSession()) {
        publishSessionUser(confirmed);
      }
      success.value = `${copy.value.success} ${confirmed.email}.`;
      showToast(success.value, "success");
      navigateWithPageProgress(props.profileHref, "replace");
    }
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : copy.value.failed;
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
  <section class="auth-page auth-page--account-action" aria-labelledby="account-confirm-title">
    <div class="auth-card auth-card--account-action">
      <a class="auth-card__brand" :href="homeHref" aria-label="BeeBa">
        <img src="/brand/beeba-logo-256.webp" alt="" width="48" height="48" decoding="async" />
      </a>
      <p class="eyebrow">{{ copy.eyebrow }}</p>
      <h1 id="account-confirm-title">{{ copy.heading }}</h1>

      <form class="auth-form" @submit.prevent="submit">
        <label class="auth-field">
          <span>{{ copy.token }}</span>
          <input v-model="token" type="text" autocomplete="one-time-code" required />
        </label>
        <button class="auth-submit" type="submit" :disabled="loading">
          {{ loading ? copy.checking : copy.action }}
        </button>
      </form>

      <div v-if="success" class="message message--success">
        {{ success }}
        <span v-if="copy.detail"> {{ copy.detail }}</span>
      </div>
      <div v-if="error" class="message message--error">{{ error }}</div>
    </div>
  </section>
</template>
