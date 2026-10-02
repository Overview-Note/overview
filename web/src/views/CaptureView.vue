<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api";
import { t } from "../i18n";
import { useWorkspaceStore } from "../stores/workspace";

const route = useRoute();
const router = useRouter();
const store = useWorkspaceStore();

const title = ref("");
const url = ref("");
const text = ref("");
const folder = ref("");
const body = ref("");
const busy = ref(false);
const error = ref("");

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

async function submit() {
  error.value = "";
  busy.value = true;
  try {
    const name = slug(title.value || url.value || t("capture.untitled"));
    const path = `${folder.value ? folder.value.replace(/\/+$/, "") + "/" : ""}${name}.md`;
    const created = await api.saveNote(path, buildBody(), "*");
    await store.refreshTree();
    router.replace({ name: "note", params: { path: created.path } });
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
}

onMounted(() => {
  title.value = (route.query.title as string) || "";
  url.value = (route.query.url as string) || "";
  text.value = (route.query.text as string) || "";
  folder.value = (route.query.folder as string) || "";
  body.value = "";
  void store.refreshTree().catch(() => undefined);
});
</script>

<template>
  <div class="capture-page">
    <form class="capture-card" @submit.prevent="submit">
      <h1>{{ t("capture.title") }}</h1>
      <label>
        {{ t("capture.noteTitle") }}
        <input v-model="title" autofocus :placeholder="t('capture.untitled')" />
      </label>
      <label>
        {{ t("capture.folder") }}
        <input v-model="folder" :placeholder="t('capture.folderHint')" />
      </label>
      <label>
        {{ t("capture.url") }}
        <input v-model="url" type="url" placeholder="https://" />
      </label>
      <label>
        {{ t("capture.text") }}
        <textarea v-model="text" rows="8"></textarea>
      </label>
      <p v-if="error" class="auth-error">{{ error }}</p>
      <div class="capture-actions">
        <button class="primary" type="submit" :disabled="busy">
          {{ busy ? t("capture.saving") : t("capture.save") }}
        </button>
        <button type="button" @click="router.replace({ name: 'home' })">
          {{ t("capture.cancel") }}
        </button>
      </div>
    </form>
  </div>
</template>
