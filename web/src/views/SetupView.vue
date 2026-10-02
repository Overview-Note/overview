<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { t } from "../i18n";
import BrandMark from "../components/BrandMark.vue";
import { useAuthStore } from "../stores/auth";

const auth = useAuthStore();
const router = useRouter();

const username = ref("");
const password = ref("");
const confirm = ref("");
const error = ref("");
const busy = ref(false);

async function submit() {
  error.value = "";
  if (password.value !== confirm.value) {
    error.value = t("auth.mismatch");
    return;
  }
  busy.value = true;
  try {
    await auth.setup(username.value, password.value);
    router.replace("/");
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
      <p class="auth-sub">{{ t("auth.setupSub") }}</p>
      <label>
        {{ t("auth.username") }}
        <input v-model="username" autocomplete="username" autofocus />
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
        {{ busy ? t("auth.setupBusy") : t("auth.setupSubmit") }}
      </button>
    </form>
  </div>
</template>
