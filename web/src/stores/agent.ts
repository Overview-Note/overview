import { defineStore } from "pinia";
import { computed, ref } from "vue";
import {
  api,
  aiErrorKey,
  type AgentPending,
  type AgentResult,
  type AgentStep,
} from "../api";
import { t } from "../i18n";

export type AgentStatus =
  | "idle"
  | "running"
  | "awaiting"
  | "done"
  | "max_steps"
  | "error";

function messageFor(e: unknown): string {
  const key = aiErrorKey(e);
  if (key !== "agent.error") return t(key);
  return e instanceof Error && e.message ? e.message : t("agent.error");
}

export const useAgentStore = defineStore("agent", () => {
  const runId = ref("");
  const status = ref<AgentStatus>("idle");
  const text = ref("");
  const steps = ref<AgentStep[]>([]);
  const pending = ref<AgentPending | null>(null);
  const error = ref("");
  const busy = ref(false);

  const wrote = computed(() =>
    steps.value.some(
      (s) => s.status === "ok" && (s.risk === "write" || s.risk === "dangerous"),
    ),
  );

  function apply(res: AgentResult) {
    runId.value = res.runId ?? "";
    text.value = res.text ?? "";
    steps.value = res.steps ?? [];
    pending.value = res.pending ?? null;
    if (res.status === "needs_confirmation") status.value = "awaiting";
    else if (res.status === "max_steps") status.value = "max_steps";
    else if (res.status === "error") status.value = "error";
    else status.value = "done";
  }

  async function start(
    input: string,
    context?: { path?: string; selection?: string },
  ): Promise<AgentResult | null> {
    const trimmed = input.trim();
    if (busy.value || !trimmed) return null;
    busy.value = true;
    error.value = "";
    status.value = "running";
    try {
      const res = await api.aiAgent({
        runId: runId.value || undefined,
        input: trimmed,
        context,
      });
      apply(res);
      return res;
    } catch (e) {
      status.value = "error";
      error.value = messageFor(e);
      return null;
    } finally {
      busy.value = false;
    }
  }

  async function confirm(
    decision: "approve" | "reject",
    args?: unknown,
  ): Promise<AgentResult | null> {
    const p = pending.value;
    if (busy.value || !runId.value || !p) return null;
    busy.value = true;
    error.value = "";
    try {
      const res = await api.aiAgentConfirm({
        runId: runId.value,
        toolCallId: p.toolCallId,
        decision,
        args,
      });
      apply(res);
      return res;
    } catch (e) {
      status.value = "error";
      error.value = messageFor(e);
      return null;
    } finally {
      busy.value = false;
    }
  }

  async function stop(): Promise<void> {
    const id = runId.value;
    if (!id) return;
    try {
      await api.aiAgentStop({ runId: id });
    } catch {
      /* stopping is best-effort */
    }
    runId.value = "";
    pending.value = null;
    status.value = "idle";
  }

  function reset() {
    runId.value = "";
    status.value = "idle";
    text.value = "";
    steps.value = [];
    pending.value = null;
    error.value = "";
    busy.value = false;
  }

  return {
    runId,
    status,
    text,
    steps,
    pending,
    error,
    busy,
    wrote,
    start,
    confirm,
    stop,
    reset,
  };
});
