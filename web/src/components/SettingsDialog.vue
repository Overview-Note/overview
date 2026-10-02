<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { api } from "../api";
import { locale, locales, setLocale, t, type Locale } from "../i18n";
import { useAuthStore } from "../stores/auth";
import type { FontSize, ThemeMode } from "../stores/settings";
import { useSettingsStore } from "../stores/settings";
import { useWorkspaceStore } from "../stores/workspace";

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ (e: "close"): void; (e: "tokens"): void }>();

const settings = useSettingsStore();
const auth = useAuthStore();
const store = useWorkspaceStore();

const importInput = ref<HTMLInputElement | null>(null);
const importBusy = ref(false);
const importResult = ref("");
const importError = ref("");

const bookmarklet = computed(() => {
  const base = `${location.origin}/capture`;
  const js =
    `javascript:(function(){` +
    `var q='?url='+encodeURIComponent(location.href)` +
    `+'&title='+encodeURIComponent(document.title)` +
    `+'&text='+encodeURIComponent((window.getSelection&&String(window.getSelection()))||'');` +
    `window.open('${base}'+q,'_blank');` +
    `})();`;
  return js;
});

async function copyBookmarklet() {
  await navigator.clipboard?.writeText(bookmarklet.value).catch(() => undefined);
}

async function downloadExport() {
  const link = document.createElement("a");
  link.href = api.exportArchiveURL();
  link.download = "";
  document.body.appendChild(link);
  link.click();
  link.remove();
}

async function onPickImport(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (!file) return;
  importBusy.value = true;
  importResult.value = "";
  importError.value = "";
  try {
    const res = await api.importArchive(file);
    importResult.value = t("settings.importDone", {
      notes: res.notes,
      assets: res.assets,
    });
    await store.refreshTree();
  } catch (e) {
    importError.value = (e as Error).message;
  } finally {
    importBusy.value = false;
  }
}

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

        <section class="settings-section">
          <h4>{{ t("settings.capture") }}</h4>
          <p class="settings-hint">{{ t("settings.captureHint") }}</p>
          <div class="settings-save">
            <button class="primary" @click="copyBookmarklet">
              {{ t("settings.copyBookmarklet") }}
            </button>
          </div>
        </section>

        <section v-if="isAdmin" class="settings-section">
          <h4>{{ t("settings.data") }}</h4>
          <p class="settings-hint">{{ t("settings.dataHint") }}</p>
          <div class="settings-save">
            <button class="primary" @click="downloadExport">
              {{ t("settings.export") }}
            </button>
            <button :disabled="importBusy" @click="importInput?.click()">
              {{ importBusy ? t("settings.importing") : t("settings.import") }}
            </button>
          </div>
          <p v-if="importResult" class="status-ok">{{ importResult }}</p>
          <p v-if="importError" class="auth-error">{{ importError }}</p>
          <input
            ref="importInput"
            type="file"
            accept=".zip,application/zip"
            hidden
            @change="onPickImport"
          />
        </section>

        <section v-if="isAdmin" class="settings-section">
          <h4>{{ t("tokens.title") }}</h4>
          <p class="settings-hint">{{ t("settings.tokensHint") }}</p>
          <div class="settings-save">
            <button class="primary" @click="emit('tokens')">
              {{ t("settings.manageTokens") }}
            </button>
          </div>
        </section>

        <footer class="settings-footer">
          <button class="primary" @click="emit('close')">
            {{ t("dialog.confirm") }}
          </button>
        </footer>      </div>
    </div>
  </Teleport>
</template>
