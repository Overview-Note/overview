<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import type { SearchHit, TreeNode } from "../api";
import { useDialogStore } from "../stores/dialog";
import { useWorkspaceStore } from "../stores/workspace";
import TreeNodeItem from "./TreeNodeItem.vue";

const store = useWorkspaceStore();
const dialogs = useDialogStore();
const router = useRouter();
const route = useRoute();

const query = ref("");
const results = ref<SearchHit[]>([]);
const searching = ref(false);
let timer: number | undefined;

const activePath = computed(() =>
  route.name === "note" ? String(route.params.path ?? "") : null,
);

function onSearchInput() {
  window.clearTimeout(timer);
  const q = query.value.trim();
  if (!q) {
    results.value = [];
    return;
  }
  timer = window.setTimeout(async () => {
    searching.value = true;
    try {
      results.value = await store.search(q);
    } catch (e) {
      store.error = (e as Error).message;
    } finally {
      searching.value = false;
    }
  }, 250);
}

function open(path: string) {
  query.value = "";
  results.value = [];
  router.push({ name: "note", params: { path } });
}

async function createNote(parent: string) {
  const name = await dialogs.ask("新建笔记", "未命名");
  if (!name) return;
  try {
    const created = await store.createNote(parent, name);
    router.push({ name: "note", params: { path: created.path } });
  } catch (e) {
    await dialogs.askConfirm("创建失败", (e as Error).message, false);
  }
}

async function createFolder(parent: string) {
  const name = await dialogs.ask("新建文件夹", "新文件夹");
  if (!name) return;
  try {
    await store.createFolder(parent, name);
  } catch (e) {
    await dialogs.askConfirm("创建失败", (e as Error).message, false);
  }
}

async function renameNode(node: TreeNode) {
  const name = await dialogs.ask("重命名", node.name);
  if (!name || name === node.name) return;
  try {
    const to = await store.renameNode(node, name);
    if (activePath.value === node.path) {
      router.push({ name: "note", params: { path: to } });
    }
  } catch (e) {
    await dialogs.askConfirm("重命名失败", (e as Error).message, false);
  }
}

async function deleteNode(node: TreeNode) {
  const ok = await dialogs.askConfirm("删除", `确定删除「${node.name}」？`);
  if (!ok) return;
  if (activePath.value === node.path) {
    router.replace({ name: "home" });
  }
  try {
    await store.removeNode(node);
  } catch (e) {
    await dialogs.askConfirm("删除失败", (e as Error).message, false);
  }
}
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar-actions">
      <button title="新建笔记" @click="createNote('')">＋ 笔记</button>
      <button title="新建文件夹" @click="createFolder('')">＋ 文件夹</button>
    </div>
    <input
      v-model="query"
      class="search"
      type="search"
      placeholder="搜索…"
      @input="onSearchInput"
    />
    <div v-if="query" class="search-results">
      <div v-if="searching" class="muted">搜索中…</div>
      <div v-for="r in results" :key="r.id" class="result" @click="open(r.path)">
        <div class="result-title">{{ r.title || r.path }}</div>
        <div class="result-path">{{ r.path }}</div>
        <div class="result-snippet" v-html="r.snippet"></div>
      </div>
      <div v-if="!searching && results.length === 0" class="muted">无结果</div>
    </div>
    <nav v-else class="tree">
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
      <div v-if="store.tree.length === 0" class="muted">还没有内容，点击上方新建</div>
    </nav>
  </aside>
</template>
