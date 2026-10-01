<script setup lang="ts">
import { ref } from "vue";
import type { SuggestionKeyDownProps, SuggestionProps } from "@tiptap/suggestion";
import type { SlashItem } from "../editor/slash";

const open = ref(false);
const items = ref<SlashItem[]>([]);
const selected = ref(0);
const pos = ref({ left: 0, top: 0 });

let commandFn: ((item: SlashItem) => void) | null = null;

function setPos(rect?: DOMRect | null) {
  if (rect) pos.value = { left: rect.left, top: rect.bottom + 6 };
}

function onStart(props: SuggestionProps<SlashItem>) {
  commandFn = props.command as (item: SlashItem) => void;
  items.value = props.items;
  selected.value = 0;
  setPos(props.clientRect?.());
  open.value = true;
}

function onUpdate(props: SuggestionProps<SlashItem>) {
  commandFn = props.command as (item: SlashItem) => void;
  items.value = props.items;
  if (selected.value >= items.value.length) selected.value = 0;
  setPos(props.clientRect?.());
  open.value = true;
}

function onExit() {
  open.value = false;
}

function onKeyDown(props: SuggestionKeyDownProps): boolean {
  if (!open.value || items.value.length === 0) return false;
  const { event } = props;
  if (event.key === "ArrowUp") {
    selected.value = (selected.value - 1 + items.value.length) % items.value.length;
    return true;
  }
  if (event.key === "ArrowDown") {
    selected.value = (selected.value + 1) % items.value.length;
    return true;
  }
  if (event.key === "Enter") {
    pick(items.value[selected.value]);
    return true;
  }
  if (event.key === "Escape") {
    open.value = false;
    return true;
  }
  return false;
}

function pick(item: SlashItem | undefined) {
  if (!item) return;
  commandFn?.(item);
  open.value = false;
}

defineExpose({ onStart, onUpdate, onExit, onKeyDown });
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open && items.length"
      class="slash-menu"
      :style="{ left: pos.left + 'px', top: pos.top + 'px' }"
    >
      <button
        v-for="(item, index) in items"
        :key="item.title"
        type="button"
        :class="{ active: index === selected }"
        @mousedown.prevent="pick(item)"
        @mouseenter="selected = index"
      >
        <span class="slash-title">{{ item.title }}</span>
        <span class="slash-hint">{{ item.hint }}</span>
      </button>
    </div>
  </Teleport>
</template>
