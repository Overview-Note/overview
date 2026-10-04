<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api } from "../api";
import { t } from "../i18n";
import BrandMark from "../components/BrandMark.vue";

const router = useRouter();

const token = ref("");
const password = ref("");
const confirm = ref("");
const error = ref("");
const done = ref(false);
const busy = ref(false);

function readToken(): string {
  const hash = window.location.hash.replace(/^#/, "");
  const fromHash = new URLSearchParams(hash).get("token");
  if (fromHash) return fromHash;
  return new URLSearchParams(window.location.search).get("token") ?? "";
}

onMounted(() => {
  token.value = readToken();
  if (!token.value) error.value = t("auth.tokenInvalid");
});

async function submit() {
  if (!token.value) {
    error.value = t("auth.tokenInvalid");
    return;
  }
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
    await api.resetPassword(token.value, password.value);
    done.value = true;
    window.setTimeout(() => router.replace({ name: "login" }), 800);
  } catch {
    error.value = t("auth.tokenInvalid");
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
      <p class="auth-sub">{{ t("auth.resetSub") }}</p>
      <template v-if="token">
        <label>
          {{ t("auth.password") }}
          <input
            v-model="password"
            type="password"
            autocomplete="new-password"
            autofocus
          />
        </label>
        <label>
          {{ t("auth.confirmPassword") }}
          <input v-model="confirm" type="password" autocomplete="new-password" />
        </label>
        <p v-if="error" class="auth-error">{{ error }}</p>
        <p v-if="done" class="status-ok">{{ t("auth.resetDone") }}</p>
        <button class="primary" type="submit" :disabled="busy || done">
          {{ busy ? t("auth.resetBusy") : t("auth.resetSubmit") }}
        </button>
      </template>
      <template v-else>
        <p class="auth-error">{{ t("auth.tokenInvalid") }}</p>
        <RouterLink class="auth-alt" :to="{ name: 'login' }">
          {{ t("auth.backToLogin") }}
        </RouterLink>
      </template>
    </form>
  </div>
</template>
