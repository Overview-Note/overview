import { defineStore } from "pinia";
import { computed, ref, watch } from "vue";

export type ThemeMode = "system" | "light" | "dark";
export type FontSize = "compact" | "default" | "relaxed" | "large";

const KEY_THEME = "overview.theme";
const KEY_FONT = "overview.fontSize";
const KEY_COMPRESS = "overview.compress";
const KEY_FOCUS = "overview.focus";

export const useSettingsStore = defineStore("settings", () => {
  const theme = ref<ThemeMode>(readTheme());
  const fontSize = ref<FontSize>(readFont());
  const compressImages = ref(localStorage.getItem(KEY_COMPRESS) !== "off");
  const focusMode = ref(localStorage.getItem(KEY_FOCUS) === "on");

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

  return {
    theme,
    fontSize,
    compressImages,
    focusMode,
    resolvedTheme,
    setTheme,
    setFontSize,
    setCompress,
    setFocus,
    toggleFocus,
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
