<script setup lang="ts">
import { ref } from "vue";
import { api } from "../api";
import { t } from "../i18n";
import BrandMark from "../components/BrandMark.vue";

const email = ref("");
const error = ref("");
const done = ref(false);
const busy = ref(false);

async function submit() {
  if (!email.value.trim()) {
    error.value = t("users.emailRequired");
    return;
  }
  error.value = "";
  done.value = false;
  busy.value = true;
  try {
    await api.requestPasswordReset(email.value.trim());
    done.value = true;
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
      <p class="auth-sub">{{ t("auth.forgotSub") }}</p>
      <label>
        {{ t("users.email") }}
        <input v-model="email" type="email" autocomplete="email" autofocus />
      </label>
      <p v-if="error" class="auth-error">{{ error }}</p>
      <p v-if="done" class="status-ok">{{ t("auth.forgotDone") }}</p>
      <button class="primary" type="submit" :disabled="busy">
        {{ busy ? t("auth.forgotBusy") : t("auth.forgotSubmit") }}
      </button>
      <RouterLink class="auth-alt" :to="{ name: 'login' }">
        {{ t("auth.backToLogin") }}
      </RouterLink>
    </form>
  </div>
</template>
