<script setup lang="ts">
import { t } from "../../i18n";
import { useSettingsStore } from "../../stores/settings";

const settings = useSettingsStore();
</script>

<template>
  <div class="settings-page">
    <h2>{{ t("settings.editor") }}</h2>
    <section class="settings-card">
      <div class="settings-row">
        <span class="settings-label">{{ t("settings.compress") }}</span>
        <label class="switch">
          <input
            type="checkbox"
            :checked="settings.compressImages"
            @change="settings.setCompress(($event.target as HTMLInputElement).checked)"
          />
          <span class="track"></span>
        </label>
      </div>
      <p class="settings-hint">{{ t("settings.compressHint") }}</p>
    </section>
  </div>
</template>
