<script setup lang="ts">
import { useRouter } from "vue-router";
import { t } from "../i18n";
import { useDialogStore } from "../stores/dialog";
import { useWorkspaceStore } from "../stores/workspace";

const store = useWorkspaceStore();
const dialogs = useDialogStore();
const router = useRouter();

async function createNote() {
  const name = await dialogs.ask(t("sidebar.newNoteTitle"), t("sidebar.newNoteDefault"));
  if (!name) return;
  try {
    const created = await store.createNote("", name);
    router.push({ name: "note", params: { path: created.path } });
  } catch (e) {
    await dialogs.askConfirm(t("sidebar.createFailed"), (e as Error).message, false);
  }
}

async function createFolder() {
  const name = await dialogs.ask(
    t("sidebar.newFolderTitle"),
    t("sidebar.newFolderDefault"),
  );
  if (!name) return;
  try {
    await store.createFolder("", name);
  } catch (e) {
    await dialogs.askConfirm(t("sidebar.createFailed"), (e as Error).message, false);
  }
}
</script>

<template>
  <div class="empty">
    <div class="empty-card">
      <p class="empty-text">{{ store.error ?? t("empty.hint") }}</p>
      <div class="empty-actions">
        <button class="primary empty-new-note" @click="createNote">
          {{ t("sidebar.newNoteTitle") }}
        </button>
        <button class="empty-new-folder" @click="createFolder">
          {{ t("sidebar.newFolderTitle") }}
        </button>
      </div>
    </div>
  </div>
</template>
