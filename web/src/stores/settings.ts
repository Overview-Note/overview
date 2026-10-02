import { defineStore } from "pinia";
import { computed, ref, watch } from "vue";

export type ThemeMode = "system" | "light" | "dark";
export type FontSize = "compact" | "default" | "relaxed" | "large";

const KEY_THEME = "overview.theme";
const KEY_FONT = "overview.fontSize";
const KEY_COMPRESS = "overview.compress";
const KEY_FOCUS = "overview.focus";
const KEY_SIDEBAR = "overview.sidebarWidth";

export const SIDEBAR_MIN = 200;
export const SIDEBAR_MAX = 560;
export const SIDEBAR_DEFAULT = 268;

function clampSidebar(px: number): number {
  if (!Number.isFinite(px)) return SIDEBAR_DEFAULT;
  return Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, Math.round(px)));
}

export const useSettingsStore = defineStore("settings", () => {
  const theme = ref<ThemeMode>(readTheme());
  const fontSize = ref<FontSize>(readFont());
  const compressImages = ref(localStorage.getItem(KEY_COMPRESS) !== "off");
  const focusMode = ref(localStorage.getItem(KEY_FOCUS) === "on");
  const sidebarWidth = ref(readSidebarWidth());

  const systemDark = ref(
    window.matchMedia?.("(prefers-color-scheme: dark)").matches ?? false,
  );
  window.matchMedia?.("(prefers-color-scheme: dark)").addEventListener("change", (e) => {
    systemDark.value = e.matches;
  });

  const resolvedTheme = computed<"light" | "dark">(() =>
    theme.value === "system" ? (systemDark.value ? "dark" : "light") : theme.value,
  );

  watch(
    [resolvedTheme, fontSize, focusMode],
    () => {
      const root = document.documentElement;
      root.dataset.theme = resolvedTheme.value;
      root.dataset.font = fontSize.value;
      root.dataset.focus = focusMode.value ? "on" : "off";
    },
    { immediate: true },
  );

  function setTheme(next: ThemeMode) {
    theme.value = next;
    localStorage.setItem(KEY_THEME, next);
  }

  function setFontSize(next: FontSize) {
    fontSize.value = next;
    localStorage.setItem(KEY_FONT, next);
  }

  function setCompress(on: boolean) {
    compressImages.value = on;
    localStorage.setItem(KEY_COMPRESS, on ? "on" : "off");
  }

  function setFocus(on: boolean) {
    focusMode.value = on;
    localStorage.setItem(KEY_FOCUS, on ? "on" : "off");
  }

  function toggleFocus() {
    setFocus(!focusMode.value);
  }

  function setSidebarWidth(px: number) {
    sidebarWidth.value = clampSidebar(px);
    localStorage.setItem(KEY_SIDEBAR, String(sidebarWidth.value));
  }

  return {
    theme,
    fontSize,
    compressImages,
    focusMode,
    sidebarWidth,
    resolvedTheme,
    setTheme,
    setFontSize,
    setCompress,
    setFocus,
    toggleFocus,
    setSidebarWidth,
  };
});

function readTheme(): ThemeMode {
  const v = localStorage.getItem(KEY_THEME);
  return v === "light" || v === "dark" || v === "system" ? v : "system";
}

function readFont(): FontSize {
  const v = localStorage.getItem(KEY_FONT);
  return v === "compact" || v === "relaxed" || v === "large" ? v : "default";
}

function readSidebarWidth(): number {
  const stored = localStorage.getItem(KEY_SIDEBAR);
  if (stored === null) return SIDEBAR_DEFAULT;
  return clampSidebar(Number(stored));
}
