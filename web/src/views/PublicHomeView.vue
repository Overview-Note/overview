<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api, type NoteMeta } from "../api";
import { t } from "../i18n";
import { useSiteStore } from "../stores/site";

const notes = ref<NoteMeta[]>([]);
const router = useRouter();
const site = useSiteStore();

onMounted(async () => {
  try {
    notes.value = await api.publicNotes();
  } catch {
    notes.value = [];
  }
});

function open(path: string) {
  router.push({ name: "public", params: { path } });
}
</script>

<template>
  <div class="public-page">
    <header class="public-header">
      <a class="public-brand" href="/">
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
      <span class="public-sub">{{ t("public.shared") }}</span>
    </header>
    <div class="public-body">
      <h1>{{ t("public.title") }}</h1>
      <ul class="public-list">
        <li v-for="note in notes" :key="note.id">
          <button class="public-link" @click="open(note.path)">
            {{ note.title || note.path }}
          </button>
        </li>
      </ul>
      <p v-if="notes.length === 0" class="muted">{{ t("public.empty") }}</p>
    </div>
  </div>
</template>
