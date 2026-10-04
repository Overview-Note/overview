import { defineStore } from "pinia";
import { computed, ref, watch } from "vue";

export type ThemeMode = "system" | "light" | "dark";
export type FontSize = "compact" | "default" | "relaxed" | "large";

export interface AccentPreset {
  id: string;
  color: string;
}

// Presets mirror the Gridea/Notion-style palette; accent drives links, active
// navigation, focus rings and controls.
export const ACCENT_PRESETS: AccentPreset[] = [
  { id: "amber", color: "#d4870e" },
  { id: "blue", color: "#4f59f5" },
  { id: "indigo", color: "#5f79d4" },
  { id: "violet", color: "#a855f7" },
  { id: "rose", color: "#e5487b" },
  { id: "green", color: "#2f9e6e" },
  { id: "teal", color: "#14b8a6" },
  { id: "graphite", color: "#57534e" },
];

export const DEFAULT_ACCENT = "#d4870e";

const KEY_THEME = "overview.theme";
const KEY_FONT = "overview.fontSize";
const KEY_COMPRESS = "overview.compress";
const KEY_FOCUS = "overview.focus";
const KEY_SIDEBAR = "overview.sidebarWidth";
const KEY_ACCENT = "overview.accent";

export const SIDEBAR_MIN = 200;
export const SIDEBAR_MAX = 520;
export const SIDEBAR_DEFAULT = 260;

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
  const accent = ref(readAccent());

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

  watch([accent, resolvedTheme], applyAccent, { immediate: true });

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

  function setAccent(hex: string) {
    accent.value = normalizeHex(hex) ?? DEFAULT_ACCENT;
    localStorage.setItem(KEY_ACCENT, accent.value);
  }

  function resetAccent() {
    setAccent(DEFAULT_ACCENT);
  }

  function applyAccent() {
    const root = document.documentElement;
    const ramp = accentRamp(accent.value, resolvedTheme.value);
    root.style.setProperty("--accent", ramp.accent);
    root.style.setProperty("--accent-hover", ramp.hover);
    root.style.setProperty("--accent-soft", ramp.soft);
    root.style.setProperty("--accent-ring", ramp.ring);
    root.style.setProperty("--bg-active", ramp.soft);
  }

  return {
    theme,
    fontSize,
    compressImages,
    focusMode,
    sidebarWidth,
    accent,
    resolvedTheme,
    setTheme,
    setFontSize,
    setCompress,
    setFocus,
    toggleFocus,
    setSidebarWidth,
    setAccent,
    resetAccent,
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

function readAccent(): string {
  return normalizeHex(localStorage.getItem(KEY_ACCENT) ?? "") ?? DEFAULT_ACCENT;
}

// --- accent ramp -----------------------------------------------------------
// A single user-chosen hex expands into the four variables the UI consumes,
// tuned differently for light and dark so contrast stays comfortable.

interface AccentRamp {
  accent: string;
  hover: string;
  soft: string;
  ring: string;
}

function normalizeHex(input: string): string | null {
  let hex = input.trim().replace(/^#/, "");
  if (/^[0-9a-fA-F]{3}$/.test(hex)) {
    hex = hex
      .split("")
      .map((c) => c + c)
      .join("");
  }
  return /^[0-9a-fA-F]{6}$/.test(hex) ? "#" + hex.toLowerCase() : null;
}

function parseHex(hex: string): [number, number, number] {
  const h = normalizeHex(hex) ?? DEFAULT_ACCENT;
  const n = parseInt(h.slice(1), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

function toHex(rgb: [number, number, number]): string {
  return (
    "#" +
    rgb
      .map((v) => Math.round(Math.min(255, Math.max(0, v))).toString(16).padStart(2, "0"))
      .join("")
  );
}

function mix(a: string, b: string, t: number): string {
  const [r1, g1, b1] = parseHex(a);
  const [r2, g2, b2] = parseHex(b);
  return toHex([r1 + (r2 - r1) * t, g1 + (g2 - g1) * t, b1 + (b2 - b1) * t]);
}

function rgba(hex: string, a: number): string {
  const [r, g, b] = parseHex(hex);
  return `rgba(${r}, ${g}, ${b}, ${a})`;
}

function accentRamp(base: string, mode: "light" | "dark"): AccentRamp {
  if (mode === "dark") {
    const accent = mix(base, "#ffffff", 0.18);
    return {
      accent,
      hover: mix(base, "#ffffff", 0.3),
      soft: mix(base, "#1f1e1c", 0.82),
      ring: rgba(accent, 0.3),
    };
  }
  return {
    accent: base,
    hover: mix(base, "#000000", 0.14),
    soft: mix(base, "#ffffff", 0.9),
    ring: rgba(base, 0.24),
  };
}
