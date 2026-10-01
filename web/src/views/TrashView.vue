<script setup lang="ts">
import { onMounted, ref } from "vue";
import { api, type TrashEntry } from "../api";
import { t } from "../i18n";
import { useDialogStore } from "../stores/dialog";
import { useWorkspaceStore } from "../stores/workspace";

const dialogs = useDialogStore();
const store = useWorkspaceStore();
const entries = ref<TrashEntry[]>([]);

async function refresh() {
  try {
    entries.value = await api.trash();
  } catch {
    entries.value = [];
  }
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString();
}

async function restore(entry: TrashEntry) {
  try {
    await api.restoreTrash(entry.id);
    await store.refreshTree();
    await refresh();
  } catch (e) {
    await dialogs.askConfirm(t("trash.title"), (e as Error).message, false);
  }
}

async function purge(entry: TrashEntry) {
  const ok = await dialogs.askConfirm(
    t("trash.purgeTitle"),
    t("trash.purgeMessage", { name: entry.path }),
  );
  if (!ok) return;
  try {
    await api.purgeTrash(entry.id);
    await refresh();
  } catch (e) {
    await dialogs.askConfirm(t("trash.title"), (e as Error).message, false);
  }
}

onMounted(refresh);
</script>

<template>
  <div class="trash-view">
    <h2>{{ t("trash.title") }}</h2>
    <p v-if="entries.length === 0" class="muted">{{ t("trash.empty") }}</p>
    <ul class="trash-list">
      <li v-for="entry in entries" :key="entry.id" class="trash-item">
        <span class="trash-path">{{ entry.path }}</span>
        <span class="trash-time">{{ formatTime(entry.deletedAt) }}</span>
        <button @click="restore(entry)">{{ t("trash.restore") }}</button>
        <button class="danger-text" @click="purge(entry)">{{ t("trash.purge") }}</button>
      </li>
    </ul>
  </div>
</template>
