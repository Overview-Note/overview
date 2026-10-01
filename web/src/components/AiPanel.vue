<script setup lang="ts">
import { ref } from "vue";
import { api } from "../api";
import { t } from "../i18n";

const props = defineProps<{ content: string; enabled?: boolean }>();
const emit = defineEmits<{
  (e: "insert", payload: { text: string; replace: boolean }): void;
}>();

function apply(text: string, replace: boolean) {
  emit("insert", { text, replace });
}

interface ChatMessage {
  role: "user" | "assistant";
  content: string;
}

const messages = ref<ChatMessage[]>([]);
const input = ref("");
const busy = ref(false);
const error = ref("");

async function send() {
  const text = input.value.trim();
  if (!text || busy.value) return;
  input.value = "";
  error.value = "";
  messages.value.push({ role: "user", content: text });
  busy.value = true;
  try {
    const reply = await api.aiChat("chat", {
      messages: [
        {
          role: "system",
          content:
            "You are an assistant in a Markdown knowledge base. Answer concisely. Use Markdown when helpful.",
        },
        ...messages.value.map((m) => ({ role: m.role, content: m.content })),
      ],
    });
    messages.value.push({ role: "assistant", content: reply });
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
}

async function organize() {
  if (busy.value || !props.content.trim()) return;
  busy.value = true;
  error.value = "";
  try {
    const reply = await api.aiChat("organize", { content: props.content });
    apply(reply, true);
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
}

async function complete() {
  if (busy.value || !props.content.trim()) return;
  busy.value = true;
  error.value = "";
  try {
    const reply = await api.aiChat("complete", { content: props.content });
    apply(reply, false);
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <aside class="ai-panel">
    <h4>{{ t("ai.title") }}</h4>
    <p v-if="enabled === false" class="muted small">{{ t("ai.notConfiguredHint") }}</p>
    <div class="ai-actions">
      <button :disabled="busy || enabled === false" @click="organize">
        {{ t("ai.organize") }}
      </button>
      <button :disabled="busy || enabled === false" @click="complete">
        {{ t("ai.complete") }}
      </button>
    </div>
    <div class="ai-messages">
      <div v-for="(m, i) in messages" :key="i" class="ai-msg" :class="m.role">
        <pre>{{ m.content }}</pre>
        <button
          v-if="m.role === 'assistant'"
          class="ai-insert"
          @click="apply(m.content, false)"
        >
          {{ t("ai.insert") }}
        </button>
      </div>
    </div>
    <p v-if="error" class="auth-error">{{ error }}</p>
    <form class="ai-input" @submit.prevent="send">
      <textarea
        v-model="input"
        rows="2"
        :placeholder="t('ai.placeholder')"
        @keydown.enter.exact.prevent="send"
      ></textarea>
      <button class="primary" type="submit" :disabled="busy || enabled === false">
        {{ busy ? t("ai.thinking") : t("ai.send") }}
      </button>
    </form>
  </aside>
</template>
