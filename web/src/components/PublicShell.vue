<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api, type NoteMeta } from "../api";
import BrandMark from "./BrandMark.vue";
import { t } from "../i18n";
import type { DocHeading } from "../markdown/doc";
import { useSiteStore } from "../stores/site";

defineProps<{
  activePath?: string | null;
  headings?: DocHeading[];
  title?: string;
}>();

const site = useSiteStore();
const router = useRouter();
const notes = ref<NoteMeta[]>([]);

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

function headingClass(level: number): string {
  return `lvl-${Math.min(Math.max(level, 2), 4)}`;
}
</script>

<template>
  <div class="public-shell">
    <header class="public-topbar">
      <a class="public-brand" href="/public">
        <BrandMark :size="30" />
        <span>{{ site.siteTitle || "Overview" }}</span>
      </a>
      <div class="public-actions">
        <slot name="actions" />
        <a class="public-home-link" href="/public">{{ t("public.allNotes") }}</a>
      </div>
    </header>

    <div class="public-layout">
      <nav class="public-nav">
        <button
          v-for="n in notes"
          :key="n.id"
          class="public-nav-item"
          :class="{ active: n.path === activePath }"
          @click="open(n.path)"
        >
          {{ n.title || n.path }}
        </button>
      </nav>

      <main class="public-content">
        <div v-if="title" class="public-crumbs">Notes / {{ title }}</div>
        <slot />
      </main>

      <aside v-if="headings && headings.length" class="public-toc">
        <h4>{{ t("toc.onThisPage") }}</h4>
        <a
          v-for="h in headings"
          :key="h.id"
          class="public-toc-item"
          :class="headingClass(h.level)"
          :href="`#${h.id}`"
        >
          {{ h.text }}
        </a>
      </aside>
    </div>
  </div>
</template>
