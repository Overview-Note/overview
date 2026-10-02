<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import BrandMark from "./components/BrandMark.vue";
import DialogHost from "./components/DialogHost.vue";
import SearchBox from "./components/SearchBox.vue";
import SettingsDialog from "./components/SettingsDialog.vue";
import ShortcutsDialog from "./components/ShortcutsDialog.vue";
import Sidebar from "./components/Sidebar.vue";
import TokensDialog from "./components/TokensDialog.vue";
import UsersDialog from "./components/UsersDialog.vue";
import { t } from "./i18n";
import { useAuthStore } from "./stores/auth";
import { useSettingsStore } from "./stores/settings";
import { useSiteStore } from "./stores/site";
import { useWorkspaceStore } from "./stores/workspace";

const store = useWorkspaceStore();
const auth = useAuthStore();
const settings = useSettingsStore();
const site = useSiteStore();
const route = useRoute();
const router = useRouter();
const usersOpen = ref(false);
const tokensOpen = ref(false);
const settingsOpen = ref(false);
const shortcutsOpen = ref(false);
const userMenuOpen = ref(false);

const plain = computed(() => site.render || route.meta.plain === true);
const showSearch = computed(() => auth.mode !== "multi" || !!auth.user);

function isTyping(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null;
  if (!el) return false;
  const tag = el.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || el.isContentEditable;
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === "F9") {
    event.preventDefault();
    settings.toggleFocus();
    return;
  }
  if (event.key === "?" && !isTyping(event.target)) {
    event.preventDefault();
    shortcutsOpen.value = true;
    return;
  }
  if (event.key === "Escape" && settings.focusMode) {
    settings.setFocus(false);
  }
}

onMounted(async () => {
  window.addEventListener("keydown", onKeydown);
  if (!auth.loaded) {
    await auth.loadState().catch(() => undefined);
  }
  if (!site.render && (auth.mode !== "multi" || auth.user)) {
    await store.refreshTree().catch(() => undefined);
  }
});

onBeforeUnmount(() => window.removeEventListener("keydown", onKeydown));

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
            <BrandMark :size="30" />
            <span class="brand-name">Overview</span>
          </RouterLink>
        </div>

        <div class="topbar-center">
          <SearchBox v-if="showSearch" />
        </div>

        <nav class="topbar-right">
          <button
            class="topbar-link"
            :class="{ on: settings.focusMode }"
            :title="t('shortcuts.focus')"
            @click="settings.toggleFocus()"
          >
            {{ t("settings.focus") }}
          </button>
          <button
            class="topbar-link"
            :title="t('shortcuts.title')"
            @click="shortcutsOpen = true"
          >
            ?
          </button>
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
    <SettingsDialog
      :open="settingsOpen"
      @close="settingsOpen = false"
      @tokens="tokensOpen = true"
    />
    <ShortcutsDialog :open="shortcutsOpen" @close="shortcutsOpen = false" />
    <TokensDialog :open="tokensOpen" @close="tokensOpen = false" />
  </div>
</template>
