<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { t } from "../i18n";
import { useAuthStore } from "../stores/auth";

const auth = useAuthStore();
const route = useRoute();
const router = useRouter();

const isAdmin = computed(() => auth.user?.role === "admin");

interface NavItem {
  name: string;
  labelKey: string;
  admin?: boolean;
}

const allItems: NavItem[] = [
  { name: "settings-appearance", labelKey: "settings.appearance" },
  { name: "settings-editor", labelKey: "settings.editor" },
  { name: "settings-ai", labelKey: "ai.title" },
  { name: "settings-mail", labelKey: "mail.title", admin: true },
  { name: "settings-site", labelKey: "site.title", admin: true },
  { name: "settings-data", labelKey: "settings.data", admin: true },
  { name: "settings-users", labelKey: "users.title", admin: true },
  { name: "settings-tokens", labelKey: "tokens.title", admin: true },
];

const items = computed(() => allItems.filter((i) => !i.admin || isAdmin.value));
const activeName = computed(() => route.name);

const SETTINGS_RETURN_KEY = "overview.settingsReturn";

function goBack() {
  let target = "";
  try {
    target = sessionStorage.getItem(SETTINGS_RETURN_KEY) ?? "";
  } catch {
    /* ignore */
  }
  if (!target || target.startsWith("/settings")) {
    const back = (window.history.state as { back?: string } | null)?.back ?? "";
    target = back && !back.startsWith("/settings") ? back : "/";
  }
  try {
    sessionStorage.removeItem(SETTINGS_RETURN_KEY);
  } catch {
    /* ignore */
  }
  router.push(target);
}
</script>

<template>
  <div class="settings-view">
    <aside class="settings-nav">
      <button class="settings-back" @click="goBack">
        <span aria-hidden="true">←</span> {{ t("settings.back") }}
      </button>
      <h2 class="settings-nav-title">{{ t("settings.title") }}</h2>
      <nav class="settings-nav-list">
        <RouterLink
          v-for="item in items"
          :key="item.name"
          class="settings-nav-item"
          :class="{ active: activeName === item.name }"
          :to="{ name: item.name }"
        >
          {{ t(item.labelKey) }}
        </RouterLink>
      </nav>
    </aside>
    <section class="settings-body">
      <RouterView />
    </section>
  </div>
</template>
