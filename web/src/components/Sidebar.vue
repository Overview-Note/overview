<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import type { TreeNode } from "../api";
import { t } from "../i18n";
import { useDialogStore } from "../stores/dialog";
import {
  SIDEBAR_DEFAULT,
  SIDEBAR_MAX,
  SIDEBAR_MIN,
  useSettingsStore,
} from "../stores/settings";
import { useWorkspaceStore } from "../stores/workspace";
import TreeNodeItem from "./TreeNodeItem.vue";

defineProps<{ open?: boolean }>();
const emit = defineEmits<{ (e: "close"): void }>();

const store = useWorkspaceStore();
const dialogs = useDialogStore();
const settings = useSettingsStore();
const router = useRouter();
const route = useRoute();

const resizing = ref(false);
const liveWidth = ref(settings.sidebarWidth);
const sidebarWidth = computed(() => (resizing.value ? liveWidth.value : settings.sidebarWidth));

function startResize(event: PointerEvent) {
  event.preventDefault();
  const startX = event.clientX;
  const startWidth = settings.sidebarWidth;
  liveWidth.value = startWidth;
  resizing.value = true;
  document.body.classList.add("resizing");

  const move = (e: PointerEvent) => {
    liveWidth.value = Math.min(
      SIDEBAR_MAX,
      Math.max(SIDEBAR_MIN, startWidth + (e.clientX - startX)),
    );
  };
  const stop = () => {
    window.removeEventListener("pointermove", move);
    window.removeEventListener("pointerup", stop);
    document.body.classList.remove("resizing");
    settings.setSidebarWidth(liveWidth.value);
    resizing.value = false;
  };
  window.addEventListener("pointermove", move);
  window.addEventListener("pointerup", stop);
}

const activePath = computed(() =>
  route.name === "note" ? String(route.params.path ?? "") : null,
);

function openNote(path: string) {
  emit("close");
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

async function moveNode(node: TreeNode, target: string) {
  try {
    const to = await store.moveNode(node, target);
    if (activePath.value === node.path) {
      router.push({ name: "note", params: { path: to } });
    }
  } catch (e) {
    await dialogs.askConfirm(t("sidebar.moveFailed"), (e as Error).message, false);
  }
}
</script>

<template>
  <aside
    class="sidebar"
    :class="{ open }"
    :style="{ '--sidebar-w': `${sidebarWidth}px` }"
  >
    <nav class="tree">
      <TreeNodeItem
        v-for="node in store.tree"
        :key="node.path"
        :node="node"
        :active-path="activePath"
        @open="openNote"
        @create-note="createNote"
        @create-folder="createFolder"
        @rename="renameNode"
        @delete="deleteNode"
        @move="moveNode"
      />
      <div v-if="store.tree.length === 0" class="muted">{{ t("sidebar.empty") }}</div>
    </nav>
    <slot />
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
    <div
      class="sidebar-resizer"
      role="separator"
      aria-orientation="vertical"
      :aria-valuenow="Math.round(sidebarWidth)"
      :aria-valuemin="SIDEBAR_MIN"
      :aria-valuemax="SIDEBAR_MAX"
      :title="t('sidebar.resize')"
      @pointerdown="startResize"
      @dblclick="settings.setSidebarWidth(SIDEBAR_DEFAULT)"
    />
  </aside>
</template>
