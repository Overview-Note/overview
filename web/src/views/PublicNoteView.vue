<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { api, type PublicNote } from "../api";
import PublicShell from "../components/PublicShell.vue";
import { t } from "../i18n";
import { renderPublicDoc, type DocHeading } from "../markdown/doc";
import { renderMermaid } from "../markdown/diagrams";
import { useAuthStore } from "../stores/auth";
import { useSiteStore } from "../stores/site";

const props = defineProps<{ path: string }>();

const auth = useAuthStore();
const site = useSiteStore();
const router = useRouter();

const note = ref<PublicNote | null>(null);
const error = ref("");
const docEl = ref<HTMLElement | null>(null);

// The "back to editor" action is only exposed to signed-in users (or when the
// instance runs without authentication). Anonymous readers never see it.
const canEdit = computed(() => !site.render && (auth.mode !== "multi" || !!auth.user));

function openEditor() {
  if (!note.value) return;
  router.push({ name: "note", params: { path: note.value.path } });
}

const rendered = computed(() => (note.value ? renderPublicDoc(note.value.body) : null));
const html = computed(() => rendered.value?.html ?? "");
const headings = computed<DocHeading[]>(() => rendered.value?.headings ?? []);

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
      await nextTick();
      void renderMermaid(docEl.value);
    } catch (e) {
      error.value = (e as Error).message;
    }
  },
  { immediate: true },
);
</script>

<template>
  <PublicShell
    :active-path="props.path"
    :headings="headings"
    :title="note ? note.title || note.path : ''"
  >
    <template #actions>
      <button v-if="canEdit && note" class="public-action-btn" @click="openEditor">
        {{ t("public.edit") }}
      </button>
    </template>
    <article v-if="note" class="public-doc">
      <h1 v-if="showTitle">{{ note.title || note.path }}</h1>
      <div ref="docEl" class="doc-body tiptap-content" v-html="html"></div>
    </article>
    <p v-else-if="error" class="muted">{{ t("public.notFound") }}</p>
  </PublicShell>
</template>
