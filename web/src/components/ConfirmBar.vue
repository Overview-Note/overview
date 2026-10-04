<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type { AgentPending } from "../api";
import { t } from "../i18n";

const props = defineProps<{ pending: AgentPending; busy?: boolean }>();
const emit = defineEmits<{
  (e: "decide", decision: "approve" | "reject", args?: unknown): void;
}>();

const editing = ref(false);
const draft = ref("{}");
const invalid = ref(false);

function initialArgs(): string {
  const value = props.pending.args;
  if (value === undefined || value === null) return "{}";
  if (typeof value === "string") {
    try {
      return JSON.stringify(JSON.parse(value), null, 2);
    } catch {
      return value;
    }
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return "{}";
  }
}

watch(
  () => props.pending.toolCallId,
  () => {
    editing.value = false;
    invalid.value = false;
    draft.value = initialArgs();
  },
  { immediate: true },
);

const danger = computed(() => props.pending.risk === "dangerous");

function approve() {
  if (editing.value && draft.value.trim()) {
    try {
      const parsed = JSON.parse(draft.value);
      invalid.value = false;
      emit("decide", "approve", parsed);
      return;
    } catch {
      invalid.value = true;
      return;
    }
  }
  emit("decide", "approve");
}
</script>

<template>
  <div class="confirm-bar" :class="{ danger }">
    <div class="confirm-preview">{{ pending.preview || pending.name }}</div>
    <div class="confirm-tool">
      <span class="tool-name">{{ pending.name }}</span>
      <span class="badge" :class="`risk-${pending.risk}`">
        {{ t(`agent.risk.${pending.risk}`) }}
      </span>
    </div>
    <button class="confirm-edit" @click="editing = !editing">
      {{ editing ? t("agent.hideArgs") : t("agent.editArgs") }}
    </button>
    <textarea
      v-if="editing"
      v-model="draft"
      class="confirm-args"
      rows="4"
      spellcheck="false"
    ></textarea>
    <p v-if="invalid" class="auth-error">{{ t("agent.invalidArgs") }}</p>
    <div class="confirm-actions">
      <button class="approve" :class="{ danger }" :disabled="busy" @click="approve">
        {{ t("agent.approve") }}
      </button>
      <button :disabled="busy" @click="emit('decide', 'reject')">
        {{ t("agent.reject") }}
      </button>
    </div>
  </div>
</template>
