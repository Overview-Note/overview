<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import DialogHost from "./components/DialogHost.vue";
import SettingsDialog from "./components/SettingsDialog.vue";
import Sidebar from "./components/Sidebar.vue";
import UsersDialog from "./components/UsersDialog.vue";
import { t } from "./i18n";
import { useAuthStore } from "./stores/auth";
import { useWorkspaceStore } from "./stores/workspace";

const store = useWorkspaceStore();
const auth = useAuthStore();
const route = useRoute();
const router = useRouter();
const usersOpen = ref(false);
const settingsOpen = ref(false);
const userMenuOpen = ref(false);

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
        <div class="brand">
          <span class="brand-mark">O</span>
          <span class="brand-name">Overview</span>
        </div>
        <div class="hint" v-if="store.loadingTree">{{ t("app.loading") }}</div>
        <div class="topbar-right">
          <button
            class="icon-btn"
            :title="t('settings.title')"
            @click="settingsOpen = true"
          >
            ⚙
          </button>
          <div v-if="auth.mode === 'multi' && auth.user" class="user-menu">
            <button class="user-trigger" @click="userMenuOpen = !userMenuOpen">
              <span class="avatar">{{
                auth.user.username.slice(0, 1).toUpperCase()
              }}</span>
              <span class="username">{{ auth.user.username }}</span>
            </button>
            <div
              v-if="userMenuOpen"
              class="user-dropdown"
              @mouseleave="userMenuOpen = false"
            >
              <button
                v-if="auth.user"
                @click="
                  router.push({ name: 'trash' });
                  userMenuOpen = false;
                "
              >
                {{ t("trash.link") }}
              </button>
              <button
                v-if="auth.user?.role === 'admin'"
                @click="
                  usersOpen = true;
                  userMenuOpen = false;
                "
              >
                {{ t("topbar.users") }}
              </button>
              <button @click="logout">{{ t("topbar.logout") }}</button>
            </div>
          </div>
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
    <SettingsDialog :open="settingsOpen" @close="settingsOpen = false" />
  </div>
</template>
