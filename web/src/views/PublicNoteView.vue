<script setup lang="ts">
import { computed, watch, ref } from "vue";
import { marked } from "marked";
import { api, type PublicNote } from "../api";
import { t } from "../i18n";
import { resolveAssetSrc } from "../markdown/assets";
import { useSiteStore } from "../stores/site";

const props = defineProps<{ path: string }>();
const site = useSiteStore();

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

// Avoid a duplicated title when the body already starts with an H1.
const showTitle = computed(() => {
  if (!note.value) return false;
  return !/^\s*#\s/.test(note.value.body);
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
    <header class="public-header">
      <a class="public-brand" href="/public">
        <svg
          viewBox="0 0 24 24"
          width="18"
          height="18"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <circle cx="12" cy="12" r="9" />
          <path d="M3 12h18M12 3c2.5 2.5 2.5 15 0 18M12 3c-2.5 2.5-2.5 15 0 18" />
        </svg>
        <span>{{ site.siteTitle }}</span>
      </a>
      <a class="public-back" href="/public">{{ t("public.allNotes") }}</a>
    </header>
    <div class="public-body">
      <article v-if="note" class="public-article">
        <h1 v-if="showTitle">{{ note.title || note.path }}</h1>
        <div class="tiptap-content" v-html="html"></div>
      </article>
      <p v-else-if="error" class="muted">{{ t("public.notFound") }}</p>
    </div>
  </div>
</template>
