<script setup lang="ts">
import { computed } from "vue";
import TokensPanel from "../../components/TokensPanel.vue";
import { t } from "../../i18n";
import { useAuthStore } from "../../stores/auth";

const auth = useAuthStore();
const isAdmin = computed(() => auth.isAdmin);
</script>

<template>
  <div class="settings-page">
    <h2>{{ t("tokens.title") }}</h2>
    <section class="settings-card">
      <template v-if="isAdmin">
        <p class="settings-hint">{{ t("settings.tokensHint") }}</p>
        <TokensPanel />
      </template>
      <p v-else class="settings-hint">{{ t("settings.adminOnly") }}</p>
    </section>
  </div>
</template>
