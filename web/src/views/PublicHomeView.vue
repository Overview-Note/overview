<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api, type NoteMeta } from "../api";
import { t } from "../i18n";

const notes = ref<NoteMeta[]>([]);
const router = useRouter();

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
      <div class="brand">Overview</div>
      <div class="public-sub">{{ t("public.shared") }}</div>
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
