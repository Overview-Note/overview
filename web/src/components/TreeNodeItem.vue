<script setup lang="ts">
import { ref } from "vue";
import type { TreeNode } from "../api";

const props = defineProps<{
  node: TreeNode;
  activePath: string | null;
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
      :class="{ active: isActive(), folder: isFolder }"
      @click="onClick"
    >
      <span class="twist" v-if="isFolder">{{ expanded ? "▾" : "▸" }}</span>
      <span class="twist" v-else></span>
      <span class="label">{{ node.title || node.name }}</span>
      <span class="row-actions" @click.stop>
        <template v-if="isFolder">
          <button title="在此新建笔记" @click="emit('create-note', node.path)">＋</button>
          <button title="在此新建文件夹" @click="emit('create-folder', node.path)">
            ⊞
          </button>
        </template>
        <button title="重命名" @click="emit('rename', node)">✎</button>
        <button title="删除" @click="emit('delete', node)">✕</button>
      </span>
    </div>
    <div v-if="isFolder && expanded" class="children">
      <TreeNodeItem
        v-for="child in node.children"
        :key="child.path"
        :node="child"
        :active-path="activePath"
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
