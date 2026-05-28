<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { getCurrentUser, login, register } from "../../lib/api/auth";
import { clearAuthSession, publishAuthSession, readAuthSession } from "../../lib/auth/session";
import { showToast } from "../../lib/ui/toast";

const props = defineProps<{
  locale: "en" | "ru";
  homeHref: string;
  profileHref: string;
  redirectHref: string;
}>();

const mode = ref<"login" | "register">("login");
const email = ref("");
const username = ref("");
const displayName = ref("");
const password = ref("");
const loading = ref(false);
const error = ref("");
const notice = ref("");
const isAlreadySignedIn = ref(false);

const copy = computed(() => {
  if (props.locale === "ru") {
    return {
      title: mode.value === "login" ? "Вход" : "Регистрация",
      email: "Email",
      username: "Username",
      displayName: "Display name",
      password: "Пароль",
      loginAction: "Войти",
      registerAction: "Создать аккаунт",
      working: "Обработка...",
      needAccount: "Нет аккаунта?",
      haveAccount: "Уже есть аккаунт?",
      switchToRegister: "Зарегистрироваться",
      switchToLogin: "Войти",
      google: "Google",
      github: "GitHub",
      soon: "soon",
      signedIn: "Вы уже вошли в аккаунт.",
      openProfile: "Открыть профиль",
      created: "Аккаунт создан. Выполняю вход.",
      loginOk: "Вход выполнен для текущей вкладки.",
      failed: "Ошибка авторизации.",
    };
  }
  return {
    title: mode.value === "login" ? "Login" : "Sign up",
    email: "Email",
    username: "Username",
    displayName: "Display name",
    password: "Password",
    loginAction: "Login",
    registerAction: "Create account",
    working: "Working...",
    needAccount: "Need an account?",
    haveAccount: "Already have an account?",
    switchToRegister: "Sign up",
    switchToLogin: "Login",
    google: "Google",
    github: "GitHub",
    soon: "soon",
    signedIn: "You are already signed in.",
    openProfile: "Open profile",
    created: "Account created. Signing in.",
    loginOk: "Signed in for this browser tab.",
    failed: "Authentication failed.",
  };
});

const submitLabel = computed(() => {
  if (loading.value) return copy.value.working;
  return mode.value === "login" ? copy.value.loginAction : copy.value.registerAction;
});

onMounted(async () => {
  try {
    await getCurrentUser();
    isAlreadySignedIn.value = true;
    window.location.replace(props.redirectHref);
    return;
  } catch {
    if (readAuthSession()) {
      clearAuthSession();
    }
    isAlreadySignedIn.value = false;
  }
});

async function submitAuth() {
  error.value = "";
  notice.value = "";
  loading.value = true;
  try {
    if (mode.value === "register") {
      await register({
        email: email.value.trim(),
        username: username.value.trim(),
        display_name: displayName.value.trim() || undefined,
        password: password.value,
      });
      notice.value = copy.value.created;
    }
    const session = await login({ email: email.value.trim(), password: password.value });
    publishAuthSession(session);
    await getCurrentUser();
    isAlreadySignedIn.value = true;
    notice.value = copy.value.loginOk;
    showToast(notice.value, { kind: "success" });
    window.location.assign(props.redirectHref);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : copy.value.failed;
    showToast(error.value, { kind: "error" });
  } finally {
    loading.value = false;
  }
}

function switchMode(nextMode: "login" | "register") {
  mode.value = nextMode;
  error.value = "";
  notice.value = "";
}

</script>

<template>
  <section class="auth-page" aria-labelledby="auth-page-title">
    <div class="auth-card">
      <a class="auth-card__brand" :href="homeHref" aria-label="BeeBa home">
        <img src="/brand/beeba-logo-256.webp" alt="" width="48" height="48" decoding="async" />
      </a>
      <h1 id="auth-page-title">{{ copy.title }}</h1>

      <div v-if="isAlreadySignedIn" class="auth-card__session">
        <p>{{ copy.signedIn }}</p>
        <a class="button button--primary" :href="profileHref">{{ copy.openProfile }}</a>
      </div>

      <form v-else class="auth-form" @submit.prevent="submitAuth">
        <label class="auth-field">
          <span>{{ copy.email }}</span>
          <input v-model="email" type="email" autocomplete="email" :placeholder="copy.email" required />
        </label>
        <label v-if="mode === 'register'" class="auth-field">
          <span>{{ copy.username }}</span>
          <input v-model="username" type="text" autocomplete="username" minlength="3" maxlength="32" :placeholder="copy.username" required />
        </label>
        <label v-if="mode === 'register'" class="auth-field">
          <span>{{ copy.displayName }}</span>
          <input v-model="displayName" type="text" maxlength="80" :placeholder="copy.displayName" />
        </label>
        <label class="auth-field">
          <span>{{ copy.password }}</span>
          <input v-model="password" type="password" :autocomplete="mode === 'register' ? 'new-password' : 'current-password'" minlength="12" :placeholder="copy.password" required />
        </label>
        <button class="auth-submit" type="submit" :disabled="loading">{{ submitLabel }}</button>
      </form>

      <div class="auth-social" aria-label="External sign-in providers">
        <button type="button" disabled>
          <img class="auth-social__icon" src="/icons/auth/google-g.svg" alt="" loading="lazy" />
          <span>{{ copy.google }}</span>
          <small>{{ copy.soon }}</small>
        </button>
        <button type="button" disabled>
          <img class="auth-social__icon auth-social__icon--github" src="/icons/auth/github-mark.svg" alt="" loading="lazy" />
          <span>{{ copy.github }}</span>
          <small>{{ copy.soon }}</small>
        </button>
      </div>

      <p class="auth-switch">
        <template v-if="mode === 'login'">
          {{ copy.needAccount }}
          <button type="button" @click="switchMode('register')">{{ copy.switchToRegister }}</button>
        </template>
        <template v-else>
          {{ copy.haveAccount }}
          <button type="button" @click="switchMode('login')">{{ copy.switchToLogin }}</button>
        </template>
      </p>

      <div v-if="error" class="message message--error">{{ error }}</div>
      <div v-if="notice" class="message message--success">{{ notice }}</div>
    </div>
  </section>
</template>
