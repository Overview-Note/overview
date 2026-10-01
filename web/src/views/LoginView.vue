<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { t } from "../i18n";
import { useAuthStore } from "../stores/auth";

const auth = useAuthStore();
const router = useRouter();
const route = useRoute();

const username = ref("");
const password = ref("");
const error = ref("");
const busy = ref(false);

async function submit() {
  error.value = "";
  busy.value = true;
  try {
    await auth.login(username.value, password.value);
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
      <h1>Overview</h1>
      <p class="auth-sub">{{ t("auth.loginSub") }}</p>
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
    </form>
  </div>
</template>
