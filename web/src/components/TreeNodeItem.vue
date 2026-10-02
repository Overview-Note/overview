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
}>();

const expanded = ref(true);
const isFolder = props.node.type === "folder";
const isActive = () => props.node.path === props.activePath;

function onClick() {
  if (isFolder) expanded.value = !expanded.value;
  else emit("open", props.node.path);
}
</script>

<template>
  <div class="tree-node">
    <div
      class="tree-row"
      :class="[
        { active: isActive(), folder: isFolder },
        isFolder
          ? `depth-${Math.min(depth ?? 0, 2)}`
          : `note depth-${Math.min((depth ?? 0) + 1, 3)}`,
      ]"
      @click="onClick"
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
      />
    </div>
  </div>
</template>

<script lang="ts">
export default { name: "TreeNodeItem" };
</script>
