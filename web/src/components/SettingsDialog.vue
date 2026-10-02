<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { api } from "../api";
import { locale, locales, setLocale, t, type Locale } from "../i18n";
import { useAuthStore } from "../stores/auth";
import type { FontSize, ThemeMode } from "../stores/settings";
import { useSettingsStore } from "../stores/settings";

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ (e: "close"): void }>();

const settings = useSettingsStore();
const auth = useAuthStore();

const aiEnabled = ref(false);
const aiModel = ref("");
const aiBaseUrl = ref("");
const aiApiKey = ref("");
const aiHasKey = ref(false);
const aiSaving = ref(false);
const aiSaved = ref(false);

const isAdmin = computed(() => auth.user?.role === "admin");

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

async function loadAI() {
  try {
    const status = await api.aiStatus();
    aiEnabled.value = status.enabled;
    aiModel.value = status.model ?? "";
  } catch {
    aiEnabled.value = false;
  }
  if (isAdmin.value) {
    try {
      const cfg = await api.aiSettings();
      aiBaseUrl.value = cfg.baseUrl;
      aiModel.value = cfg.model;
      aiHasKey.value = cfg.hasKey;
    } catch {
      /* ignore */
    }
  }
}

async function saveAI() {
  aiSaving.value = true;
  aiSaved.value = false;
  try {
    const cfg = await api.saveAISettings({
      baseUrl: aiBaseUrl.value,
      apiKey: aiApiKey.value,
      model: aiModel.value,
    });
    aiHasKey.value = cfg.hasKey;
    aiApiKey.value = "";
    aiSaved.value = true;
    await loadAI();
  } finally {
    aiSaving.value = false;
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) void loadAI();
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

          <template v-if="isAdmin">
            <label class="settings-field">
              <span>{{ t("settings.aiBaseUrl") }}</span>
              <input v-model="aiBaseUrl" placeholder="https://api.openai.com/v1" />
            </label>
            <label class="settings-field">
              <span>{{ t("settings.aiApiKey") }}</span>
              <input
                v-model="aiApiKey"
                type="password"
                :placeholder="aiHasKey ? t('settings.aiKeySet') : 'sk-...'"
              />
            </label>
            <label class="settings-field">
              <span>{{ t("settings.aiModelField") }}</span>
              <input v-model="aiModel" placeholder="gpt-4o-mini" />
            </label>
            <p class="settings-hint">{{ t("settings.aiHint") }}</p>
            <div class="settings-save">
              <button class="primary" :disabled="aiSaving" @click="saveAI">
                {{ aiSaving ? t("settings.saving") : t("settings.save") }}
              </button>
              <span v-if="aiSaved" class="status-ok">{{ t("settings.saved") }}</span>
            </div>
          </template>
          <p v-else class="settings-hint">{{ t("settings.aiAdminOnly") }}</p>
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
