<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import DialogHost from "./components/DialogHost.vue";
import SearchBox from "./components/SearchBox.vue";
import SettingsDialog from "./components/SettingsDialog.vue";
import Sidebar from "./components/Sidebar.vue";
import UsersDialog from "./components/UsersDialog.vue";
import { t } from "./i18n";
import { useAuthStore } from "./stores/auth";
import { useSiteStore } from "./stores/site";
import { useWorkspaceStore } from "./stores/workspace";

const store = useWorkspaceStore();
const auth = useAuthStore();
const site = useSiteStore();
const route = useRoute();
const router = useRouter();
const usersOpen = ref(false);
const settingsOpen = ref(false);
const userMenuOpen = ref(false);

const plain = computed(() => site.render || route.meta.plain === true);
const showSearch = computed(() => auth.mode !== "multi" || !!auth.user);

onMounted(async () => {
  if (!auth.loaded) {
    await auth.loadState().catch(() => undefined);
  }
  if (!site.render && (auth.mode !== "multi" || auth.user)) {
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
  userMenuOpen.value = false;
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
        <div class="topbar-left">
          <RouterLink class="brand" :to="{ name: 'home' }">
            <span class="brand-mark">O</span>
            <span class="brand-name">Overview</span>
          </RouterLink>
        </div>

        <div class="topbar-center">
          <SearchBox v-if="showSearch" />
        </div>

        <nav class="topbar-right">
          <button class="topbar-link" @click="settingsOpen = true">
            {{ t("settings.title") }}
          </button>
          <button
            v-if="auth.user"
            class="topbar-link"
            @click="router.push({ name: 'trash' })"
          >
            {{ t("trash.link") }}
          </button>
          <button
            v-if="auth.user?.role === 'admin'"
            class="topbar-link"
            @click="usersOpen = true"
          >
            {{ t("topbar.users") }}
          </button>
          <div v-if="auth.mode === 'multi' && auth.user" class="user-menu">
            <button class="user-trigger" @click="userMenuOpen = !userMenuOpen">
              <span class="avatar">{{
                auth.user.username.slice(0, 1).toUpperCase()
              }}</span>
            </button>
            <div
              v-if="userMenuOpen"
              class="user-dropdown"
              @mouseleave="userMenuOpen = false"
            >
              <div class="user-dropdown-name">{{ auth.user.username }}</div>
              <button @click="logout">{{ t("topbar.logout") }}</button>
            </div>
          </div>
        </nav>
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
    <SettingsDialog :open="settingsOpen" @close="settingsOpen = false" />
  </div>
</template>
