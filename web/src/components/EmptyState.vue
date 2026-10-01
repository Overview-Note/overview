<script setup lang="ts">
import { t } from "../i18n";
import { useWorkspaceStore } from "../stores/workspace";

const store = useWorkspaceStore();
</script>

<template>
  <div class="empty">
    <p>{{ store.error ?? t("empty.hint") }}</p>
  </div>
</template>
