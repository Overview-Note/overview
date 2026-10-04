<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ApiError, api } from "../api";
import { t } from "../i18n";
import BrandMark from "../components/BrandMark.vue";
import { useAuthStore } from "../stores/auth";

const auth = useAuthStore();
const router = useRouter();
const route = useRoute();

const username = ref("");
const password = ref("");
const error = ref("");
const busy = ref(false);
const notVerified = ref(false);
const resent = ref(false);

const safeLink = computed(() => {
  const link = auth.loginLink;
  if (!link || !link.url) return null;
  if (!/^https?:\/\//i.test(link.url)) return null;
  return { text: link.text || link.url, url: link.url };
});

async function submit() {
  error.value = "";
  notVerified.value = false;
  resent.value = false;
  busy.value = true;
  try {
    await auth.login(username.value, password.value);
    const redirect = (route.query.redirect as string) || "/";
    router.replace(redirect);
  } catch (e) {
    if (e instanceof ApiError && e.code === "email_not_verified") {
      notVerified.value = true;
      error.value = t("auth.emailNotVerified");
    } else {
      error.value = (e as Error).message;
    }
  } finally {
    busy.value = false;
  }
}

async function resend() {
  error.value = "";
  resent.value = false;
  try {
    await api.resendVerification(username.value.trim());
    resent.value = true;
  } catch (e) {
    error.value = (e as Error).message;
  }
}
</script>

<template>
  <div class="auth-page">
    <form class="auth-card" @submit.prevent="submit">
      <div class="auth-head">
        <BrandMark :size="42" />
        <h1>Overview</h1>
      </div>
      <p class="auth-sub">{{ t("auth.loginSub") }}</p>
      <p v-if="auth.loginHint" class="auth-hint">{{ auth.loginHint }}</p>
      <label>
        {{ t("auth.username") }}
        <input v-model="username" autocomplete="username" autofocus />
      </label>
      <label>
        {{ t("auth.password") }}
        <input v-model="password" type="password" autocomplete="current-password" />
      </label>
      <p v-if="error" class="auth-error">{{ error }}</p>
      <button class="primary" type="submit" :disabled="busy">
        {{ busy ? t("auth.loginBusy") : t("auth.loginSubmit") }}
      </button>
      <button v-if="notVerified" type="button" class="auth-alt" @click="resend">
        {{ t("auth.resendVerification") }}
      </button>
      <p v-if="resent" class="status-ok">{{ t("auth.resendDone") }}</p>
      <RouterLink
        v-if="auth.mailEnabled"
        class="auth-alt"
        :to="{ name: 'forgot-password' }"
      >
        {{ t("auth.forgot") }}
      </RouterLink>
      <RouterLink
        v-if="auth.registrationEnabled"
        class="auth-alt"
        :to="{ name: 'register' }"
      >
        {{ t("auth.register") }}
      </RouterLink>
      <div v-if="auth.loginIcp || safeLink" class="auth-foot">
        <a
          v-if="safeLink"
          :href="safeLink.url"
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ safeLink.text }}
        </a>
        <span v-if="auth.loginIcp">{{ auth.loginIcp }}</span>
      </div>
    </form>
  </div>
</template>
