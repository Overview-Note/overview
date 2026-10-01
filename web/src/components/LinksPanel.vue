<script setup lang="ts">
import { ref, watch } from "vue";
import { api, type LinksResult } from "../api";
import { t } from "../i18n";

const props = defineProps<{ path: string }>();
const emit = defineEmits<{
  (e: "navigate", path: string): void;
  (e: "create", target: string): void;
}>();

const links = ref<LinksResult>({ outgoing: [], backlinks: [] });

watch(
  () => props.path,
  async (path) => {
    if (!path) return;
    try {
      links.value = await api.links(path);
    } catch {
      links.value = { outgoing: [], backlinks: [] };
    }
  },
  { immediate: true },
);
</script>

<template>
  <aside class="links-panel">
    <section v-if="links.backlinks.length">
      <h4>
        {{ t("links.backlinks") }} <span class="count">{{ links.backlinks.length }}</span>
      </h4>
      <button
        v-for="item in links.backlinks"
        :key="item.id"
        class="link-item"
        @click="emit('navigate', item.path)"
      >
        {{ item.title || item.path }}
      </button>
    </section>

    <section v-if="links.outgoing.length">
      <h4>
        {{ t("links.outgoing") }} <span class="count">{{ links.outgoing.length }}</span>
      </h4>
      <template v-for="link in links.outgoing" :key="link.raw">
        <button
          v-if="link.target"
          class="link-item"
          @click="emit('navigate', link.target.path)"
        >
          {{ link.target.title || link.target.path }}
        </button>
        <button v-else class="link-item missing" @click="emit('create', link.raw)">
          {{ link.raw }} <span class="badge">{{ t("links.missing") }}</span>
        </button>
      </template>
    </section>

    <div v-if="!links.backlinks.length && !links.outgoing.length" class="muted small">
      {{ t("links.empty") }}
    </div>
  </aside>
</template>
