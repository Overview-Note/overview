<script setup lang="ts">
import { ref, watch } from "vue";
import { api, type APIToken } from "../api";
import { t } from "../i18n";
import { useDialogStore } from "../stores/dialog";

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ (e: "close"): void }>();

const dialogs = useDialogStore();
const tokens = ref<APIToken[]>([]);
const error = ref("");
const secret = ref("");
const copied = ref(false);

async function refresh() {
  error.value = "";
  try {
    tokens.value = await api.listTokens();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      secret.value = "";
      copied.value = false;
      void refresh();
    }
  },
);

async function addToken() {
  error.value = "";
  const name = await dialogs.ask(t("tokens.namePrompt"), "");
  if (!name) return;
  const days = await dialogs.ask(t("tokens.expiryPrompt"), "");
  if (days === null) return;
  const trimmed = days.trim();
  const expiresDays = /^\d+$/.test(trimmed) ? Number(trimmed) : 0;
  try {
    const res = await api.createToken(name, expiresDays);
    secret.value = res.secret;
    await refresh();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function revoke(token: APIToken) {
  const ok = await dialogs.askConfirm(
    t("tokens.revokeTitle"),
    t("tokens.revokeMessage", { name: token.name }),
  );
  if (!ok) return;
  try {
    await api.deleteToken(token.id);
    await refresh();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function copySecret() {
  try {
    await navigator.clipboard?.writeText(secret.value);
    copied.value = true;
    window.setTimeout(() => (copied.value = false), 1500);
  } catch {
    /* clipboard unavailable */
  }
}

function fmt(value?: string): string {
  if (!value) return t("tokens.never");
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? value : d.toLocaleString();
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="overlay" @click.self="emit('close')">
      <div class="dialog dialog-wide">
        <h3>{{ t("tokens.title") }}</h3>

        <div v-if="secret" class="token-secret">
          <p><strong>{{ t("tokens.secretTitle") }}</strong> — {{ t("tokens.secretHint") }}</p>
          <div class="token-secret-row">
            <code>{{ secret }}</code>
            <button @click="copySecret">
              {{ copied ? t("tokens.copied") : t("tokens.copy") }}
            </button>
          </div>
        </div>

        <div class="user-list">
          <div v-for="token in tokens" :key="token.id" class="user-row">
            <span class="user-name">{{ token.name }}</span>
            <span class="badge">{{ token.prefix }}…</span>
            <span class="muted small">
              {{ t("tokens.created") }} {{ fmt(token.created) }}
            </span>
            <span class="muted small">
              {{ t("tokens.lastUsed") }} {{ fmt(token.lastUsed) }}
            </span>
            <button class="danger-text" @click="revoke(token)">
              {{ t("tokens.revoke") }}
            </button>
          </div>
          <p v-if="tokens.length === 0" class="muted">{{ t("tokens.empty") }}</p>
        </div>
        <p v-if="error" class="auth-error">{{ error }}</p>
        <div class="dialog-actions">
          <button @click="addToken">{{ t("tokens.add") }}</button>
          <button class="primary" @click="emit('close')">{{ t("tokens.close") }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
