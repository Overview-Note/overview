<script setup lang="ts">
import { nextTick, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { api, type TreeNode } from "../api";
import { t } from "../i18n";
import { useWorkspaceStore } from "../stores/workspace";

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ (e: "close"): void }>();

const router = useRouter();
const store = useWorkspaceStore();

const title = ref("");
const url = ref("");
const text = ref("");
const folder = ref("");
const busy = ref(false);
const fetching = ref(false);
const error = ref("");
const folders = ref<{ path: string; label: string }[]>([]);
const urlInput = ref<HTMLInputElement | null>(null);

function slug(s: string): string {
  return (
    s
      .trim()
      .replace(/[\\/:*?"<>|]+/g, " ")
      .replace(/\s+/g, " ")
      .slice(0, 80) || t("capture.untitled")
  );
}

function buildBody(): string {
  const parts = [`# ${title.value || t("capture.untitled")}`];
  if (text.value) parts.push("", text.value);
  if (url.value) parts.push("", `> 来源：[${url.value}](${url.value})`);
  return parts.join("\n") + "\n";
}

function collectFolders(
  nodes: TreeNode[],
  depth: number,
  acc: { path: string; label: string }[],
) {
  for (const node of nodes) {
    if (node.type !== "folder") continue;
    acc.push({ path: node.path, label: `${"  ".repeat(depth)}${node.title || node.name}` });
    if (node.children?.length) collectFolders(node.children, depth + 1, acc);
  }
}

async function loadFolders() {
  try {
    await store.refreshTree();
    const acc: { path: string; label: string }[] = [];
    collectFolders(store.tree, 0, acc);
    folders.value = acc;
  } catch {
    /* ignore tree load failures */
  }
}

async function fetchPreview() {
  const target = url.value.trim();
  if (!target) return;
  error.value = "";
  fetching.value = true;
  try {
    const preview = await api.capturePreview(target);
    if (preview.title && !title.value.trim()) title.value = preview.title;
    if (preview.text && !text.value.trim()) text.value = preview.text;
  } catch (e) {
    error.value = (e as Error).message || t("capture.fetchFailed");
  } finally {
    fetching.value = false;
  }
}

async function submit() {
  error.value = "";
  busy.value = true;
  try {
    const name = slug(title.value || url.value || t("capture.untitled"));
    const path = `${folder.value ? folder.value.replace(/\/+$/, "") + "/" : ""}${name}.md`;
    const created = await api.saveNote(path, buildBody(), "*");
    await store.refreshTree();
    emit("close");
    router.push({ name: "note", params: { path: created.path } });
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
}

function close() {
  if (busy.value) return;
  emit("close");
}

watch(
  () => props.open,
  (open) => {
    if (!open) return;
    title.value = "";
    url.value = "";
    text.value = "";
    folder.value = "";
    error.value = "";
    busy.value = false;
    fetching.value = false;
    void loadFolders();
    nextTick(() => urlInput.value?.focus());
  },
);
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="overlay" @click.self="close">
      <div class="dialog dialog-wide capture-dialog">
        <header class="settings-header">
          <h3>{{ t("capture.title") }}</h3>
          <button class="icon-btn" @click="close" aria-label="close">✕</button>
        </header>
        <form class="capture-form" @submit.prevent="submit">
          <label>
            {{ t("capture.url") }}
            <span class="capture-fetch">
              <input
                ref="urlInput"
                v-model="url"
                type="url"
                placeholder="https://"
                @keydown.enter.prevent="fetchPreview"
              />
              <button
                class="primary"
                type="button"
                :disabled="fetching || !url.trim()"
                @click="fetchPreview"
              >
                {{ fetching ? t("capture.fetching") : t("capture.fetch") }}
              </button>
            </span>
          </label>
          <p class="capture-hint">{{ t("capture.previewHint") }}</p>
          <label>
            {{ t("capture.noteTitle") }}
            <input v-model="title" :placeholder="t('capture.untitled')" />
          </label>
          <label>
            {{ t("capture.folder") }}
            <select v-model="folder">
              <option value="">{{ t("capture.folderRoot") }}</option>
              <option v-for="f in folders" :key="f.path" :value="f.path">{{ f.label }}</option>
            </select>
          </label>
          <label>
            {{ t("capture.text") }}
            <textarea v-model="text" rows="8"></textarea>
          </label>
          <p v-if="error" class="auth-error">{{ error }}</p>
          <div class="dialog-actions">
            <button type="button" @click="close">{{ t("capture.cancel") }}</button>
            <button class="primary" type="submit" :disabled="busy">
              {{ busy ? t("capture.saving") : t("capture.save") }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </Teleport>
</template>
