<script setup lang="ts">
import { computed, ref } from "vue";
import { extractPaths } from "../agent/paths";
import type { AIStatus } from "../api";
import { t } from "../i18n";
import { useAgentStore } from "../stores/agent";
import ConfirmBar from "./ConfirmBar.vue";
import ToolCallCard from "./ToolCallCard.vue";

const props = defineProps<{
  path?: string;
  selection?: string;
  status?: AIStatus | null;
}>();
const emit = defineEmits<{
  (e: "insert", payload: { text: string; replace: boolean }): void;
  (e: "changed", payload: { wrote: boolean; paths: string[] }): void;
}>();

const agent = useAgentStore();
const input = ref("");
const lastInput = ref("");

const availability = computed<"ok" | "disabled" | "unsupported" | "unconfigured">(
  () => {
    const s = props.status;
    if (!s || !s.enabled) return "unconfigured";
    if (!s.agentEnabled) return "disabled";
    if (s.toolCalling === false) return "unsupported";
    return "ok";
  },
);
const usable = computed(() => availability.value === "ok");

const selectionPreview = computed(() => {
  const value = props.selection?.trim() ?? "";
  return value.length > 40 ? `${value.slice(0, 40)}…` : value;
});

const runDisabled = computed(
  () => !usable.value || agent.busy || !input.value.trim() || agent.pending !== null,
);

function context() {
  const ctx: { path?: string; selection?: string } = {};
  if (props.path) ctx.path = props.path;
  if (props.selection && props.selection.trim()) ctx.selection = props.selection;
  return ctx;
}

function changed() {
  const paths = agent.steps
    .filter(
      (s) => s.status === "ok" && (s.risk === "write" || s.risk === "dangerous"),
    )
    .flatMap((s) => extractPaths(s.args, s.summary));
  emit("changed", { wrote: agent.wrote, paths });
}

async function run() {
  const text = input.value.trim();
  if (!text || !usable.value || agent.busy || agent.pending) return;
  lastInput.value = text;
  input.value = "";
  const res = await agent.start(text, context());
  if (res) changed();
}

async function decide(decision: "approve" | "reject", args?: unknown) {
  const res = await agent.confirm(decision, args);
  if (res) changed();
}

function stop() {
  void agent.stop();
}

function retry() {
  if (!lastInput.value) return;
  input.value = lastInput.value;
  void run();
}
</script>

<template>
  <aside class="agent-panel">
    <h4>{{ t("agent.title") }}</h4>

    <p v-if="availability === 'unconfigured'" class="muted small">
      {{ t("ai.notConfiguredHint") }}
    </p>
    <p v-else-if="availability === 'disabled'" class="muted small">
      {{ t("agent.disabled") }}
    </p>
    <p v-else-if="availability === 'unsupported'" class="muted small">
      {{ t("agent.unsupported") }}
    </p>

    <div v-if="path || selectionPreview" class="agent-context">
      <span v-if="path" class="agent-chip" :title="path">
        {{ t("agent.contextNote") }}: {{ path }}
      </span>
      <span v-if="selectionPreview" class="agent-chip" :title="selection">
        {{ t("agent.contextSelection") }}: {{ selectionPreview }}
      </span>
    </div>

    <div v-if="agent.steps.length" class="agent-steps">
      <ToolCallCard v-for="s in agent.steps" :key="s.toolCallId" :step="s" />
    </div>

    <div v-if="agent.text" class="agent-text">
      <pre>{{ agent.text }}</pre>
      <button
        class="ai-insert"
        @click="emit('insert', { text: agent.text, replace: false })"
      >
        {{ t("ai.insert") }}
      </button>
    </div>

    <ConfirmBar
      v-if="agent.pending"
      :pending="agent.pending"
      :busy="agent.busy"
      @decide="decide"
    />

    <p v-if="agent.error" class="auth-error">{{ agent.error }}</p>
    <p v-else-if="agent.status === 'running'" class="agent-status">
      {{ t("agent.running") }}
    </p>
    <p v-else-if="agent.status === 'awaiting'" class="agent-status">
      {{ t("agent.awaiting") }}
    </p>
    <p v-else-if="agent.status === 'max_steps'" class="agent-status">
      {{ t("agent.maxSteps") }}
    </p>
    <p v-else-if="agent.status === 'done'" class="agent-status">
      {{ t("agent.done") }}
    </p>

    <form class="ai-input" @submit.prevent="run">
      <textarea
        v-model="input"
        rows="2"
        :placeholder="t('agent.placeholder')"
        :disabled="!usable"
        @keydown.enter.exact.prevent="run"
      ></textarea>
      <div class="agent-actions">
        <template v-if="agent.status === 'running'">
          <button type="button" class="agent-stop" @click="stop">
            {{ t("agent.stop") }}
          </button>
        </template>
        <template v-else>
          <button
            v-if="agent.status === 'error' && lastInput"
            type="button"
            :disabled="agent.busy"
            @click="retry"
          >
            {{ t("agent.retry") }}
          </button>
          <button class="primary" type="submit" :disabled="runDisabled">
            {{ agent.busy ? t("agent.running") : t("agent.run") }}
          </button>
        </template>
      </div>
    </form>

    <button
      v-if="agent.steps.length || agent.text || agent.error"
      class="agent-clear"
      @click="agent.reset()"
    >
      {{ t("agent.clear") }}
    </button>
  </aside>
</template>
