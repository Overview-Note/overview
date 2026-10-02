<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api, type NoteMeta } from "../api";
import PublicShell from "../components/PublicShell.vue";
import { t } from "../i18n";
import { useAuthStore } from "../stores/auth";
import { useSiteStore } from "../stores/site";

const notes = ref<NoteMeta[]>([]);
const router = useRouter();
const site = useSiteStore();
const auth = useAuthStore();

const canEdit = computed(() => !site.render && (auth.mode !== "multi" || !!auth.user));

function openApp() {
  router.push({ name: "home" });
}

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
  <PublicShell>
    <template #actions>
      <button v-if="canEdit" class="public-action-btn" @click="openApp">
        {{ t("public.openApp") }}
      </button>
    </template>
    <h1 class="public-home-title">{{ site.siteTitle || t("public.title") }}</h1>
    <ul class="public-note-list">
      <li v-for="note in notes" :key="note.id">
        <button class="public-note-link" @click="open(note.path)">
          {{ note.title || note.path }}
        </button>
      </li>
    </ul>
    <p v-if="notes.length === 0" class="muted">{{ t("public.empty") }}</p>
  </PublicShell>
</template>
