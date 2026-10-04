<script setup lang="ts">
import { locale, locales, setLocale, t, type Locale } from "../../i18n";
import type { FontSize, ThemeMode } from "../../stores/settings";
import {
  ACCENT_PRESETS,
  DEFAULT_ACCENT,
  useSettingsStore,
} from "../../stores/settings";

const settings = useSettingsStore();

const themes: { value: ThemeMode; labelKey: string }[] = [
  { value: "system", labelKey: "settings.themeSystem" },
  { value: "light", labelKey: "settings.themeLight" },
  { value: "dark", labelKey: "settings.themeDark" },
];

const fonts: { value: FontSize; labelKey: string }[] = [
  { value: "compact", labelKey: "settings.fontCompact" },
  { value: "default", labelKey: "settings.fontDefault" },
  { value: "relaxed", labelKey: "settings.fontRelaxed" },
  { value: "large", labelKey: "settings.fontLarge" },
];

const accentPresets = ACCENT_PRESETS;
const defaultAccent = DEFAULT_ACCENT;
</script>

<template>
  <div class="settings-page">
    <h2>{{ t("settings.appearance") }}</h2>
    <section class="settings-card">
      <div class="settings-row">
        <span class="settings-label">{{ t("settings.theme") }}</span>
        <div class="segmented">
          <button
            v-for="item in themes"
            :key="item.value"
            :class="{ on: settings.theme === item.value }"
            @click="settings.setTheme(item.value)"
          >
            {{ t(item.labelKey) }}
          </button>
        </div>
      </div>
      <div class="settings-row">
        <span class="settings-label">{{ t("settings.fontSize") }}</span>
        <div class="segmented">
          <button
            v-for="item in fonts"
            :key="item.value"
            :class="{ on: settings.fontSize === item.value }"
            @click="settings.setFontSize(item.value)"
          >
            {{ t(item.labelKey) }}
          </button>
        </div>
      </div>
      <div class="settings-row">
        <span class="settings-label">{{ t("settings.language") }}</span>
        <div class="segmented">
          <button
            v-for="item in locales"
            :key="item.value"
            :class="{ on: locale === item.value }"
            @click="setLocale(item.value as Locale)"
          >
            {{ item.label }}
          </button>
        </div>
      </div>
      <div class="settings-row settings-row-col">
        <span class="settings-label">{{ t("settings.accent") }}</span>
        <div class="accent-picker">
          <button
            v-for="preset in accentPresets"
            :key="preset.id"
            class="accent-swatch"
            :class="{ on: settings.accent === preset.color }"
            :style="{ background: preset.color }"
            :title="preset.id"
            @click="settings.setAccent(preset.color)"
          ></button>
          <label class="accent-swatch accent-custom" :title="t('settings.accentCustom')">
            <input
              type="color"
              :value="settings.accent"
              @input="settings.setAccent(($event.target as HTMLInputElement).value)"
            />
          </label>
          <button class="accent-reset" @click="settings.setAccent(defaultAccent)">
            {{ t("settings.accentReset") }}
          </button>
        </div>
      </div>
    </section>
  </div>
</template>
