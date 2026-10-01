<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import DialogHost from "./components/DialogHost.vue";
import Sidebar from "./components/Sidebar.vue";
import UsersDialog from "./components/UsersDialog.vue";
import { useAuthStore } from "./stores/auth";
import { useWorkspaceStore } from "./stores/workspace";

const store = useWorkspaceStore();
const auth = useAuthStore();
const route = useRoute();
const router = useRouter();
const usersOpen = ref(false);

const plain = computed(() => route.meta.plain === true);

onMounted(async () => {
  if (!auth.loaded) {
    await auth.loadState().catch(() => undefined);
  }
  if (auth.mode !== "multi" || auth.user) {
    await store.refreshTree().catch(() => undefined);
  }
});

watch(
  () => auth.user,
  (user) => {
    if (user) void store.refreshTree().catch(() => undefined);
  },
);

async function logout() {
  await auth.logout().catch(() => undefined);
  router.replace({ name: "login" });
}
</script>

<template>
  <div class="app">
    <template v-if="plain">
      <RouterView />
    </template>
    <template v-else>
      <header class="topbar">
        <div class="brand">Overview</div>
        <div class="hint" v-if="store.loadingTree">载入中…</div>
        <div class="topbar-right">
          <span v-if="auth.mode === 'multi' && auth.user" class="username">
            {{ auth.user.username }}
          </span>
          <button v-if="auth.user?.role === 'admin'" @click="usersOpen = true">
            用户管理
          </button>
          <button v-if="auth.mode === 'multi' && auth.user" @click="logout">退出</button>
        </div>
      </header>
      <div class="layout">
        <Sidebar />
        <main class="content">
          <RouterView />
        </main>
      </div>
    </template>
    <DialogHost />
    <UsersDialog :open="usersOpen" @close="usersOpen = false" />
  </div>
</template>
