<script setup lang="ts">
import { t } from "../i18n";

defineProps<{ open: boolean }>();
const emit = defineEmits<{ (e: "close"): void }>();

const groups: { titleKey: string; items: { keys: string; labelKey: string }[] }[] = [
  {
    titleKey: "shortcuts.groupGlobal",
    items: [
      { keys: "Ctrl / Cmd + K", labelKey: "shortcuts.search" },
      { keys: "F9", labelKey: "shortcuts.focus" },
      { keys: "?", labelKey: "shortcuts.help" },
    ],
  },
  {
    titleKey: "shortcuts.groupEditing",
    items: [
      { keys: "Ctrl / Cmd + B", labelKey: "shortcuts.bold" },
      { keys: "Ctrl / Cmd + I", labelKey: "shortcuts.italic" },
      { keys: "Ctrl / Cmd + E", labelKey: "shortcuts.code" },
      { keys: "/", labelKey: "shortcuts.slash" },
      { keys: "Esc", labelKey: "shortcuts.exitFocus" },
    ],
  },
];
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="overlay" @click.self="emit('close')">
      <div class="shortcuts-dialog">
        <header class="settings-header">
          <h3>{{ t("shortcuts.title") }}</h3>
          <button class="icon-btn" @click="emit('close')" aria-label="close">✕</button>
        </header>
        <section v-for="g in groups" :key="g.titleKey" class="settings-section">
          <h4>{{ t(g.titleKey) }}</h4>
          <div v-for="item in g.items" :key="item.keys" class="shortcut-row">
            <kbd>{{ item.keys }}</kbd>
            <span>{{ t(item.labelKey) }}</span>
          </div>
        </section>
      </div>
    </div>
  </Teleport>
</template>
