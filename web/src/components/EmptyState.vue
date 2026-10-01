<script setup lang="ts">
import { useWorkspaceStore } from "../stores/workspace";

const store = useWorkspaceStore();
</script>

<template>
  <div class="empty">
    <p>{{ store.error ?? "从左侧选择或新建一篇笔记" }}</p>
  </div>
</template>
