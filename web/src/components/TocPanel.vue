<script setup lang="ts">
import { t } from "../i18n";
import type { TocItem } from "../editor/toc";

defineProps<{ items: TocItem[] }>();
const emit = defineEmits<{ (e: "navigate", item: TocItem): void }>();

function indent(level: number): string {
  return `${(level - 1) * 12}px`;
}
</script>

<template>
  <aside class="toc-panel">
    <h4>{{ t("toc.title") }}</h4>
    <nav v-if="items.length">
      <button
        v-for="item in items"
        :key="item.id"
        class="toc-item"
        :class="{ active: item.isActive, scrolled: item.isScrolledOver }"
        :style="{ paddingLeft: indent(item.level) }"
        :title="item.textContent"
        @click="emit('navigate', item)"
      >
        {{ item.textContent || "（无标题）" }}
      </button>
    </nav>
    <div v-else class="muted small">{{ t("toc.empty") }}</div>
  </aside>
</template>
