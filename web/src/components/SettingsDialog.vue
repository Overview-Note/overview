<script setup lang="ts">
import { ref, watch } from "vue";
import { api } from "../api";
import { locale, locales, setLocale, t, type Locale } from "../i18n";
import type { FontSize, ThemeMode } from "../stores/settings";
import { useSettingsStore } from "../stores/settings";

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ (e: "close"): void }>();

const settings = useSettingsStore();

const aiEnabled = ref(false);
const aiModel = ref("");

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

watch(
  () => props.open,
  async (open) => {
    if (!open) return;
    try {
      const status = await api.aiStatus();
      aiEnabled.value = status.enabled;
      aiModel.value = status.model ?? "";
    } catch {
      aiEnabled.value = false;
    }
  },
);
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="overlay" @click.self="emit('close')">
      <div class="settings-dialog">
        <header class="settings-header">
          <h3>{{ t("settings.title") }}</h3>
          <button class="icon-btn" @click="emit('close')" aria-label="close">✕</button>
        </header>

        <section class="settings-section">
          <h4>{{ t("settings.appearance") }}</h4>
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
        </section>

        <section class="settings-section">
          <h4>{{ t("settings.editor") }}</h4>
          <div class="settings-row">
            <span class="settings-label">{{ t("settings.compress") }}</span>
            <label class="switch">
              <input
                type="checkbox"
                :checked="settings.compressImages"
                @change="
                  settings.setCompress(($event.target as HTMLInputElement).checked)
                "
              />
              <span class="track"></span>
            </label>
          </div>
          <p class="settings-hint">{{ t("settings.compressHint") }}</p>
        </section>

        <section class="settings-section">
          <h4>{{ t("ai.title") }}</h4>
          <div class="settings-row">
            <span class="settings-label">{{ t("settings.aiStatus") }}</span>
            <span :class="aiEnabled ? 'status-ok' : 'status-off'">
              {{ aiEnabled ? t("settings.aiEnabled") : t("settings.aiDisabled") }}
            </span>
          </div>
          <p v-if="aiEnabled" class="settings-hint">
            {{ t("settings.aiModel", { model: aiModel }) }}
          </p>
          <p v-else class="settings-hint settings-code">
            OVERVIEW_AI_BASE_URL / OVERVIEW_AI_API_KEY / OVERVIEW_AI_MODEL
          </p>
        </section>

        <footer class="settings-footer">
          <button class="primary" @click="emit('close')">
            {{ t("dialog.confirm") }}
          </button>
        </footer>
      </div>
    </div>
  </Teleport>
</template>
