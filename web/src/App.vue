<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import BrandMark from "./components/BrandMark.vue";
import CaptureDialog from "./components/CaptureDialog.vue";
import DialogHost from "./components/DialogHost.vue";
import SearchBox from "./components/SearchBox.vue";
import ShortcutsDialog from "./components/ShortcutsDialog.vue";
import Sidebar from "./components/Sidebar.vue";
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
const shortcutsOpen = ref(false);
const captureOpen = ref(false);
const searchOpen = ref(false);
const sidebarOpen = ref(false);
const userMenuOpen = ref(false);
const userMenu = ref<HTMLElement | null>(null);

const plain = computed(() => site.render || route.meta.plain === true);
const showSearch = computed(() => auth.mode !== "multi" || !!auth.user);
const isSettings = computed(() => String(route.name ?? "").startsWith("settings"));

const SETTINGS_RETURN_KEY = "overview.settingsReturn";

function openSettings() {
  if (!isSettings.value) {
    try {
      sessionStorage.setItem(SETTINGS_RETURN_KEY, route.fullPath);
    } catch {
      /* ignore */
    }
  }
  router.push({ name: "settings" });
}

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
  if (event.key === "Escape" && userMenuOpen.value) {
    userMenuOpen.value = false;
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

function onDocumentClick(event: MouseEvent) {
  if (!userMenuOpen.value) return;
  const el = userMenu.value;
  if (el && !el.contains(event.target as Node)) {
    userMenuOpen.value = false;
  }
}

onMounted(async () => {
  window.addEventListener("keydown", onKeydown);
  window.addEventListener("click", onDocumentClick);
  if (!auth.loaded) {
    await auth.loadState().catch(() => undefined);
  }
  if (!site.render && (auth.mode !== "multi" || auth.user)) {
    await store.refreshTree().catch(() => undefined);
  }
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", onKeydown);
  window.removeEventListener("click", onDocumentClick);
});

watch(
  () => auth.user,
  (user) => {
    if (user) void store.refreshTree().catch(() => undefined);
  },
);

watch(
  () => route.fullPath,
  () => {
    sidebarOpen.value = false;
    searchOpen.value = false;
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
          <button
            v-if="!isSettings"
            class="nav-toggle icon-btn"
            :aria-label="t('sidebar.menu')"
            :aria-expanded="sidebarOpen"
            @click="sidebarOpen = !sidebarOpen"
          >
            ☰
          </button>
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
          <button class="topbar-link" @click="openSettings">
            {{ t("settings.title") }}
          </button>
          <button
            v-if="showSearch"
            class="topbar-link"
            @click="captureOpen = true"
          >
            {{ t("capture.open") }}
          </button>
          <button
            v-if="auth.user"
            class="topbar-link"
            @click="router.push({ name: 'trash' })"
          >
            {{ t("trash.link") }}
          </button>
          <div
            v-if="auth.mode === 'multi' && auth.user"
            ref="userMenu"
            class="user-menu"
          >
            <button class="user-trigger" @click="userMenuOpen = !userMenuOpen">
              <span class="avatar">{{
                auth.user.username.slice(0, 1).toUpperCase()
              }}</span>
            </button>
            <div v-if="userMenuOpen" class="user-dropdown">
              <div class="user-dropdown-name">{{ auth.user.username }}</div>
              <div class="user-dropdown-sep"></div>
              <button class="user-dropdown-logout" @click="logout">
                {{ t("topbar.logout") }}
              </button>
            </div>
          </div>
        </nav>
      </header>
      <div class="layout">
        <Sidebar v-if="!isSettings" :open="sidebarOpen" @close="sidebarOpen = false">
          <div class="sidebar-drawer-nav">
            <button
              v-if="showSearch"
              class="sidebar-drawer-item"
              @click="sidebarOpen = false; searchOpen = true"
            >
              {{ t("sidebar.search") }}
            </button>
            <button
              class="sidebar-drawer-item"
              :class="{ on: settings.focusMode }"
              @click="sidebarOpen = false; settings.toggleFocus()"
            >
              {{ t("settings.focus") }}
            </button>
            <button
              class="sidebar-drawer-item"
              @click="sidebarOpen = false; shortcutsOpen = true"
            >
              {{ t("shortcuts.title") }}
            </button>
            <button class="sidebar-drawer-item" @click="openSettings">
              {{ t("settings.title") }}
            </button>
            <button
              v-if="showSearch"
              class="sidebar-drawer-item"
              @click="sidebarOpen = false; captureOpen = true"
            >
              {{ t("capture.open") }}
            </button>
            <button
              v-if="auth.user"
              class="sidebar-drawer-item"
              @click="router.push({ name: 'trash' })"
            >
              {{ t("trash.link") }}
            </button>
            <template v-if="auth.mode === 'multi' && auth.user">
              <div class="sidebar-drawer-user">
                <span class="avatar">{{
                  auth.user.username.slice(0, 1).toUpperCase()
                }}</span>
                <span class="sidebar-drawer-name">{{ auth.user.username }}</span>
              </div>
              <button class="sidebar-drawer-item" @click="logout">
                {{ t("topbar.logout") }}
              </button>
            </template>
          </div>
        </Sidebar>
        <div
          class="sidebar-backdrop"
          v-if="sidebarOpen"
          @click="sidebarOpen = false"
        ></div>
        <main class="content">
          <RouterView />
        </main>
      </div>
    </template>
    <DialogHost />
    <ShortcutsDialog :open="shortcutsOpen" @close="shortcutsOpen = false" />
    <CaptureDialog :open="captureOpen" @close="captureOpen = false" />
    <div v-if="searchOpen" class="search-overlay" @click.self="searchOpen = false">
      <div class="search-overlay-head">
        <button
          class="search-overlay-close"
          :aria-label="t('tokens.close')"
          @click="searchOpen = false"
        >
          ✕
        </button>
      </div>
      <div class="search-overlay-body">
        <SearchBox autofocus />
      </div>
    </div>
  </div>
</template>
