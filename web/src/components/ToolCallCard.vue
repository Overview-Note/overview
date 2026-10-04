<script setup lang="ts">
import { computed, ref } from "vue";
import { useRouter } from "vue-router";
import type { AgentStep } from "../api";
import { extractPaths } from "../agent/paths";
import { t } from "../i18n";

const props = defineProps<{ step: AgentStep }>();
const router = useRouter();

const open = ref(false);

function pretty(value: unknown): string {
  if (value === undefined || value === null || value === "") return "";
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (trimmed.startsWith("{") || trimmed.startsWith("[")) {
      try {
        return JSON.stringify(JSON.parse(trimmed), null, 2);
      } catch {
        /* fall through to the raw string */
      }
    }
    return value;
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

const argsText = computed(() => pretty(props.step.args));

const MAX_PATHS = 20;

const paths = computed(() =>
  extractPaths(props.step.args, props.step.summary).filter((p) => /\.md$/i.test(p)),
);
const visiblePaths = computed(() => paths.value.slice(0, MAX_PATHS));
const hiddenCount = computed(() => Math.max(0, paths.value.length - MAX_PATHS));

function go(path: string) {
  router.push({ name: "note", params: { path } });
}
</script>

<template>
  <div class="tool-card">
    <button class="tool-head" @click="open = !open">
      <span class="tool-caret">{{ open ? "▾" : "▸" }}</span>
      <span class="tool-name">{{ step.name }}</span>
      <span class="badge" :class="`risk-${step.risk}`">
        {{ t(`agent.risk.${step.risk}`) }}
      </span>
      <span class="badge" :class="`st-${step.status}`">
        {{ t(`agent.status.${step.status}`) }}
      </span>
    </button>
    <div v-if="open" class="tool-body">
      <pre v-if="argsText">{{ argsText }}</pre>
      <pre v-if="step.summary" class="tool-summary">{{ step.summary }}</pre>
      <div v-if="paths.length" class="tool-paths">
        <button v-for="p in visiblePaths" :key="p" class="tool-path" @click="go(p)">
          {{ p }}
        </button>
        <span v-if="hiddenCount" class="tool-path-more">+{{ hiddenCount }}</span>
      </div>
    </div>
  </div>
</template>
