<script setup lang="ts">
import { onMounted, ref } from "vue";
import { api, type DesktopSettings, type UpdateInfo } from "../../api";
import { t } from "../../i18n";

const settings = ref<DesktopSettings | null>(null);
const settingsError = ref("");
const saving = ref(false);

const update = ref<UpdateInfo | null>(null);
const checking = ref(false);
const updateError = ref("");

onMounted(async () => {
  try {
    settings.value = await api.desktopSettings();
  } catch (e) {
    settingsError.value = (e as Error).message;
  }
});

async function toggleAutostart() {
  if (!settings.value || saving.value) return;
  const next = !settings.value.autostart;
  saving.value = true;
  settingsError.value = "";
  try {
    settings.value = await api.saveDesktopSettings({ autostart: next });
  } catch (e) {
    settingsError.value = (e as Error).message;
  } finally {
    saving.value = false;
  }
}

async function checkUpdate() {
  if (checking.value) return;
  checking.value = true;
  updateError.value = "";
  try {
    update.value = await api.checkUpdate();
  } catch (e) {
    updateError.value = (e as Error).message;
  } finally {
    checking.value = false;
  }
}

function openDownload() {
  const url = update.value?.url;
  if (url) window.open(url, "_blank", "noopener");
}
</script>

<template>
  <div class="settings-page">
    <h2>{{ t("settings.desktop") }}</h2>
    <section class="settings-card">
      <div class="settings-row">
        <span class="settings-label">{{ t("desktop.autostart") }}</span>
        <label class="switch">
          <input
            type="checkbox"
            :checked="settings?.autostart ?? false"
            :disabled="saving || !settings"
            @change="toggleAutostart"
          />
          <span class="track"></span>
        </label>
      </div>
      <p class="settings-hint">{{ t("desktop.autostartHint") }}</p>

      <div class="settings-row">
        <span class="settings-label">{{ t("desktop.dataDir") }}</span>
        <code class="settings-code">{{ settings?.dataDir ?? "—" }}</code>
      </div>

      <div class="settings-row">
        <span class="settings-label">{{ t("desktop.version") }}</span>
        <code class="settings-code">{{ settings?.version ?? "—" }}</code>
      </div>

      <div class="settings-row">
        <span class="settings-label">{{ t("desktop.updates") }}</span>
        <div class="settings-save inline">
          <button :disabled="checking" @click="checkUpdate">
            {{ checking ? t("desktop.checking") : t("desktop.checkUpdate") }}
          </button>
          <button
            v-if="update?.hasUpdate && update.url"
            class="primary"
            @click="openDownload"
          >
            {{ t("desktop.openDownload") }}
          </button>
        </div>
      </div>
      <p v-if="update" class="settings-hint">
        {{
          update.hasUpdate
            ? t("desktop.updateAvailable", { latest: update.latest })
            : t("desktop.upToDate")
        }}
      </p>
      <p v-if="updateError" class="auth-error">{{ updateError }}</p>
      <p v-if="settingsError" class="auth-error">{{ settingsError }}</p>
      <p class="settings-hint">{{ t("desktop.hint") }}</p>
    </section>
  </div>
</template>
