<script setup lang="ts">
import { computed, ref } from "vue";
import { api } from "../../api";
import { t } from "../../i18n";
import { useAuthStore } from "../../stores/auth";
import { useWorkspaceStore } from "../../stores/workspace";

const auth = useAuthStore();
const store = useWorkspaceStore();
const isAdmin = computed(() => auth.user?.role === "admin");

const importInput = ref<HTMLInputElement | null>(null);
const importBusy = ref(false);
const importResult = ref("");
const importError = ref("");

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
</script>

<template>
  <div class="settings-page">
    <h2>{{ t("settings.data") }}</h2>
    <section class="settings-card">
      <template v-if="isAdmin">
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
      </template>
      <p v-else class="settings-hint">{{ t("settings.adminOnly") }}</p>
    </section>
  </div>
</template>
