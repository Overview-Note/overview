<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api";
import { t } from "../i18n";
import BrandMark from "../components/BrandMark.vue";
import { useAuthStore } from "../stores/auth";

const auth = useAuthStore();
const router = useRouter();
const route = useRoute();

const email = ref("");
const password = ref("");
const confirm = ref("");
const error = ref("");
const busy = ref(false);
const done = ref(false);

const safeLink = computed(() => {
  const link = auth.loginLink;
  if (!link || !link.url) return null;
  if (!/^https?:\/\//i.test(link.url)) return null;
  return { text: link.text || link.url, url: link.url };
});

async function submit() {
  if (password.value.length < 6) {
    error.value = t("auth.passwordTooShort");
    return;
  }
  if (password.value !== confirm.value) {
    error.value = t("auth.mismatch");
    return;
  }
  error.value = "";
  busy.value = true;
  try {
    const result = await api.register(email.value.trim(), password.value);
    if (result.verificationRequired) {
      done.value = true;
      return;
    }
    await auth.login(email.value.trim(), password.value);
    const redirect = (route.query.redirect as string) || "/";
    router.replace(redirect);
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    busy.value = false;
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
      <p class="auth-sub">{{ t("auth.registerSub") }}</p>
      <p v-if="auth.loginHint" class="auth-hint">{{ auth.loginHint }}</p>
      <template v-if="!done">
        <label>
          {{ t("users.email") }}
          <input v-model="email" type="email" autocomplete="email" autofocus />
        </label>
        <label>
          {{ t("auth.password") }}
          <input v-model="password" type="password" autocomplete="new-password" />
        </label>
        <label>
          {{ t("auth.confirmPassword") }}
          <input v-model="confirm" type="password" autocomplete="new-password" />
        </label>
        <p v-if="error" class="auth-error">{{ error }}</p>
        <button class="primary" type="submit" :disabled="busy">
          {{ busy ? t("auth.registerBusy") : t("auth.registerSubmit") }}
        </button>
      </template>
      <template v-else>
        <p class="status-ok">{{ t("auth.registerDone") }}</p>
        <RouterLink class="auth-alt" :to="{ name: 'login' }">
          {{ t("auth.backToLogin") }}
        </RouterLink>
      </template>
      <RouterLink
        v-if="!done && auth.registrationEnabled"
        class="auth-alt"
        :to="{ name: 'login' }"
      >
        {{ t("auth.haveAccount") }}
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
