<script setup lang="ts">
import { onMounted, ref } from "vue";
import { api } from "../api";
import { t } from "../i18n";
import BrandMark from "../components/BrandMark.vue";

type State = "busy" | "done" | "error";

const state = ref<State>("busy");

function readToken(): string {
  const hash = window.location.hash.replace(/^#/, "");
  const fromHash = new URLSearchParams(hash).get("token");
  if (fromHash) return fromHash;
  return new URLSearchParams(window.location.search).get("token") ?? "";
}

onMounted(async () => {
  const token = readToken();
  if (!token) {
    state.value = "error";
    return;
  }
  try {
    await api.verifyEmail(token);
    state.value = "done";
  } catch {
    state.value = "error";
  }
});
</script>

<template>
  <div class="auth-page">
    <div class="auth-card">
      <div class="auth-head">
        <BrandMark :size="42" />
        <h1>Overview</h1>
      </div>
      <p class="auth-sub">{{ t("auth.verifySub") }}</p>
      <p v-if="state === 'busy'" class="status-off">{{ t("auth.verifyBusy") }}</p>
      <p v-else-if="state === 'done'" class="status-ok">{{ t("auth.verifyDone") }}</p>
      <p v-else class="auth-error">{{ t("auth.tokenInvalid") }}</p>
      <RouterLink class="auth-alt" :to="{ name: 'login' }">
        {{ t("auth.backToLogin") }}
      </RouterLink>
    </div>
  </div>
</template>
