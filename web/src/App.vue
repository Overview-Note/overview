<script setup lang="ts">
import { onMounted } from "vue";
import DialogHost from "./components/DialogHost.vue";
import Sidebar from "./components/Sidebar.vue";
import { useWorkspaceStore } from "./stores/workspace";

const store = useWorkspaceStore();

onMounted(store.refreshTree);
</script>

<template>
  <div class="app">
    <header class="topbar">
      <div class="brand">Overview</div>
      <div class="hint" v-if="store.loadingTree">载入中…</div>
    </header>
    <div class="layout">
      <Sidebar />
      <main class="content">
        <RouterView />
      </main>
    </div>
    <DialogHost />
  </div>
</template>
