<script setup lang="ts">
import { computed, watch, ref } from "vue";
import { marked } from "marked";
import { api, type PublicNote } from "../api";
import { resolveAssetSrc } from "../markdown/assets";

const props = defineProps<{ path: string }>();

const note = ref<PublicNote | null>(null);
const error = ref("");

const html = computed(() => {
  if (!note.value) return "";
  // Render wiki-links as plain text for anonymous readers (no target leakage).
  const md = note.value.body.replace(/\[\[([^[\]]+?)\]\]/g, (_m, inner: string) => {
    const [target, display] = inner.split("|");
    return (display ?? target).trim() || target;
  });
  const rendered = marked.parse(md, { async: false }) as string;
  return resolveAssetSrc(rendered);
});

watch(
  () => props.path,
  async (path) => {
    error.value = "";
    note.value = null;
    try {
      note.value = await api.publicNote(path);
    } catch (e) {
      error.value = (e as Error).message;
    }
  },
  { immediate: true },
);
</script>

<template>
  <div class="public-page">
    <div class="public-body">
      <article v-if="note" class="public-article">
        <h1>{{ note.title || note.path }}</h1>
        <div class="tiptap-content" v-html="html"></div>
      </article>
      <p v-else-if="error" class="muted">{{ error }}</p>
    </div>
  </div>
</template>
