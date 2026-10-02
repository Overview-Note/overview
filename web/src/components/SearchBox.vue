<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from "vue";
import { useRouter } from "vue-router";
import type { SearchHit } from "../api";
import { t } from "../i18n";
import { useWorkspaceStore } from "../stores/workspace";

const store = useWorkspaceStore();
const router = useRouter();

const query = ref("");
const results = ref<SearchHit[]>([]);
const open = ref(false);
const searching = ref(false);
let timer: number | undefined;

function onInput() {
  window.clearTimeout(timer);
  const q = query.value.trim();
  if (!q) {
    results.value = [];
    open.value = false;
    return;
  }
  timer = window.setTimeout(async () => {
    searching.value = true;
    try {
      results.value = await store.search(q);
      open.value = true;
    } catch {
      results.value = [];
    } finally {
      searching.value = false;
    }
  }, 220);
}

function go(path: string) {
  query.value = "";
  results.value = [];
  open.value = false;
  router.push({ name: "note", params: { path } });
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === "Escape") {
    open.value = false;
    (event.target as HTMLElement)?.blur();
  }
}

function onGlobalKey(event: KeyboardEvent) {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") {
    event.preventDefault();
    input.value?.focus();
  }
}

const input = ref<HTMLInputElement | null>(null);

watch(
  () => router.currentRoute.value,
  () => {
    open.value = false;
  },
);

window.addEventListener("keydown", onGlobalKey);
onBeforeUnmount(() => window.removeEventListener("keydown", onGlobalKey));
</script>

<template>
  <div class="searchbox">
    <span class="searchbox-icon">⌕</span>
    <input
      ref="input"
      v-model="query"
      class="searchbox-input"
      type="search"
      :placeholder="t('sidebar.search')"
      @input="onInput"
      @keydown="onKeydown"
      @focus="open = !!query"
    />
    <span class="searchbox-kbd">Ctrl K</span>

    <div v-if="open" class="searchbox-results" @mouseleave="open = false">
      <div v-if="searching" class="muted">{{ t("sidebar.searching") }}</div>
      <button v-for="r in results" :key="r.id" class="result" @click="go(r.path)">
        <div class="result-title">{{ r.title || r.path }}</div>
        <div class="result-path">{{ r.path }}</div>
        <div class="result-snippet" v-html="r.snippet"></div>
      </button>
      <div v-if="!searching && results.length === 0" class="muted">
        {{ t("sidebar.noResults") }}
      </div>
    </div>
  </div>
</template>
