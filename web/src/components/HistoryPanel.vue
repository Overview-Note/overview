<script setup lang="ts">
import { ref, watch } from "vue";
import { api, type Revision } from "../api";
import { t } from "../i18n";
import { useDialogStore } from "../stores/dialog";

const props = defineProps<{ path: string }>();
const emit = defineEmits<{ (e: "restored"): void }>();

const dialogs = useDialogStore();
const revisions = ref<Revision[]>([]);
const preview = ref<string | null>(null);

function formatTime(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleString();
}

async function refresh() {
  try {
    revisions.value = await api.revisions(props.path);
  } catch {
    revisions.value = [];
  }
  preview.value = null;
}

watch(() => props.path, refresh, { immediate: true });

async function view(rev: Revision) {
  try {
    preview.value = await api.revisionContent(props.path, rev.id);
  } catch (e) {
    await dialogs.askConfirm(t("history.title"), (e as Error).message, false);
  }
}

async function restore(rev: Revision) {
  const ok = await dialogs.askConfirm(
    t("history.restoreTitle"),
    t("history.restoreMessage"),
  );
  if (!ok) return;
  try {
    await api.restoreRevision(props.path, rev.id);
    await refresh();
    emit("restored");
  } catch (e) {
    await dialogs.askConfirm(t("history.title"), (e as Error).message, false);
  }
}
</script>

<template>
  <div class="history-panel">
    <h4>{{ t("history.title") }}</h4>
    <div v-if="revisions.length === 0" class="muted small">{{ t("history.empty") }}</div>
    <ul class="history-list">
      <li v-for="rev in revisions" :key="rev.id" class="history-item">
        <button class="history-time" @click="view(rev)">
          {{ formatTime(rev.savedAt) }}
        </button>
        <button class="history-restore" @click="restore(rev)">
          {{ t("history.restore") }}
        </button>
      </li>
    </ul>
    <pre v-if="preview !== null" class="history-preview">{{ preview }}</pre>
  </div>
</template>
