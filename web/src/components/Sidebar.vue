<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import type { TreeNode } from "../api";
import { t } from "../i18n";
import { useDialogStore } from "../stores/dialog";
import { useWorkspaceStore } from "../stores/workspace";
import TreeNodeItem from "./TreeNodeItem.vue";

const store = useWorkspaceStore();
const dialogs = useDialogStore();
const router = useRouter();
const route = useRoute();

const activePath = computed(() =>
  route.name === "note" ? String(route.params.path ?? "") : null,
);

function open(path: string) {
  router.push({ name: "note", params: { path } });
}

async function createNote(parent: string) {
  const name = await dialogs.ask(t("sidebar.newNoteTitle"), t("sidebar.newNoteDefault"));
  if (!name) return;
  try {
    const created = await store.createNote(parent, name);
    router.push({ name: "note", params: { path: created.path } });
  } catch (e) {
    await dialogs.askConfirm(t("sidebar.createFailed"), (e as Error).message, false);
  }
}

async function createFolder(parent: string) {
  const name = await dialogs.ask(
    t("sidebar.newFolderTitle"),
    t("sidebar.newFolderDefault"),
  );
  if (!name) return;
  try {
    await store.createFolder(parent, name);
  } catch (e) {
    await dialogs.askConfirm(t("sidebar.createFailed"), (e as Error).message, false);
  }
}

async function renameNode(node: TreeNode) {
  const name = await dialogs.ask(t("sidebar.renameTitle"), node.name);
  if (!name || name === node.name) return;
  try {
    const to = await store.renameNode(node, name);
    if (activePath.value === node.path) {
      router.push({ name: "note", params: { path: to } });
    }
  } catch (e) {
    await dialogs.askConfirm(t("sidebar.renameFailed"), (e as Error).message, false);
  }
}

async function deleteNode(node: TreeNode) {
  const ok = await dialogs.askConfirm(
    t("sidebar.deleteTitle"),
    t("sidebar.deleteConfirm", { name: node.name }),
  );
  if (!ok) return;
  if (activePath.value === node.path) {
    router.replace({ name: "home" });
  }
  try {
    await store.removeNode(node);
  } catch (e) {
    await dialogs.askConfirm(t("sidebar.deleteFailed"), (e as Error).message, false);
  }
}
</script>

<template>
  <aside class="sidebar">
    <nav class="tree">
      <TreeNodeItem
        v-for="node in store.tree"
        :key="node.path"
        :node="node"
        :active-path="activePath"
        @open="open"
        @create-note="createNote"
        @create-folder="createFolder"
        @rename="renameNode"
        @delete="deleteNode"
      />
      <div v-if="store.tree.length === 0" class="muted">{{ t("sidebar.empty") }}</div>
    </nav>
    <div class="sidebar-foot">
      <button
        class="foot-primary"
        :title="t('sidebar.newNoteTitle')"
        @click="createNote('')"
      >
        {{ t("sidebar.newNote") }}
      </button>
      <button
        class="foot-secondary"
        :title="t('sidebar.newFolderTitle')"
        @click="createFolder('')"
      >
        {{ t("sidebar.newFolder") }}
      </button>
    </div>
  </aside>
</template>
