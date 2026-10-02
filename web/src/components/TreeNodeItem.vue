<script setup lang="ts">
import { ref } from "vue";
import type { TreeNode } from "../api";
import { t } from "../i18n";

const props = defineProps<{
  node: TreeNode;
  activePath: string | null;
  depth?: number;
}>();

const emit = defineEmits<{
  (e: "open", path: string): void;
  (e: "create-note", parent: string): void;
  (e: "create-folder", parent: string): void;
  (e: "rename", node: TreeNode): void;
  (e: "delete", node: TreeNode): void;
  (e: "move", node: TreeNode, targetFolder: string): void;
}>();

const expanded = ref(true);
const isFolder = props.node.type === "folder";
const isActive = () => props.node.path === props.activePath;
const dragOver = ref(false);

function onClick() {
  if (isFolder) expanded.value = !expanded.value;
  else emit("open", props.node.path);
}

function onDragStart(event: DragEvent) {
  if (!event.dataTransfer) return;
  event.dataTransfer.effectAllowed = "move";
  event.dataTransfer.setData("text/plain", props.node.path);
}

function onDragOver(event: DragEvent) {
  const dragged = event.dataTransfer?.types.includes("text/plain");
  if (!dragged) return;
  // A node may only be dropped into a folder, never into itself/descendant.
  if (!isFolder) return;
  event.preventDefault();
  event.dataTransfer!.dropEffect = "move";
  dragOver.value = true;
}

function onDragLeave() {
  dragOver.value = false;
}

function onDrop(event: DragEvent) {
  dragOver.value = false;
  const from = event.dataTransfer?.getData("text/plain");
  if (!from || !isFolder) return;
  event.preventDefault();
  event.stopPropagation();
  if (from === props.node.path || from.startsWith(props.node.path + "/")) return;
  emit("move", { path: from } as TreeNode, props.node.path);
}
</script>

<template>
  <div class="tree-node">
    <div
      class="tree-row"
      :class="[
        { active: isActive(), folder: isFolder, 'drag-over': dragOver },
        isFolder
          ? `depth-${Math.min(depth ?? 0, 2)}`
          : `note depth-${Math.min((depth ?? 0) + 1, 3)}`,
      ]"
      draggable="true"
      @click="onClick"
      @dragstart="onDragStart"
      @dragover="onDragOver"
      @dragleave="onDragLeave"
      @drop="onDrop"
    >
      <span class="twist" v-if="isFolder">{{ expanded ? "▾" : "▸" }}</span>
      <span class="twist" v-else></span>
      <span class="label">{{ node.title || node.name }}</span>
      <span class="row-actions" @click.stop>
        <template v-if="isFolder">
          <button :title="t('tree.newNoteHere')" @click="emit('create-note', node.path)">
            ＋
          </button>
          <button
            :title="t('tree.newFolderHere')"
            @click="emit('create-folder', node.path)"
          >
            ⊞
          </button>
        </template>
        <button :title="t('tree.rename')" @click="emit('rename', node)">✎</button>
        <button :title="t('tree.delete')" @click="emit('delete', node)">✕</button>
      </span>
    </div>
    <div v-if="isFolder && expanded" class="children">
      <TreeNodeItem
        v-for="child in node.children"
        :key="child.path"
        :node="child"
        :active-path="activePath"
        :depth="(depth ?? 0) + 1"
        @open="emit('open', $event)"
        @create-note="emit('create-note', $event)"
        @create-folder="emit('create-folder', $event)"
        @rename="emit('rename', $event)"
        @delete="emit('delete', $event)"
        @move="(n, target) => emit('move', n, target)"
      />
    </div>
  </div>
</template>

<script lang="ts">
export default { name: "TreeNodeItem" };
</script>
