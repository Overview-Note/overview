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
const name = ref("");
const expiresDays = ref("");
const busy = ref(false);

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
      name.value = "";
      expiresDays.value = "";
      error.value = "";
      void refresh();
    }
  },
);

async function generate() {
  error.value = "";
  const tokenName = name.value.trim();
  if (!tokenName) {
    error.value = t("tokens.namePrompt");
    return;
  }
  const days = expiresDays.value.trim();
  const daysNum = /^\d+$/.test(days) ? Number(days) : 0;
  busy.value = true;
  try {
    const res = await api.createToken(tokenName, daysNum);
    secret.value = res.secret;
    copied.value = false;
    name.value = "";
    expiresDays.value = "";
    await refresh();
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
}

async function copySecret() {
  if (!secret.value) return;
  try {
    await navigator.clipboard?.writeText(secret.value);
    copied.value = true;
    window.setTimeout(() => (copied.value = false), 1500);
  } catch {
    /* clipboard unavailable */
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

function fmt(value?: string): string {
  if (!value) return t("tokens.never");
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? value : d.toLocaleString();
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="overlay" @click.self="emit('close')">
      <div class="dialog dialog-tokens">
        <h3>{{ t("tokens.title") }}</h3>

        <div v-if="secret" class="token-secret">
          <p class="token-secret-head">{{ t("tokens.secretTitle") }}</p>
          <p class="token-hint">{{ t("tokens.secretHint") }}</p>
          <div class="token-secret-row">
            <code>{{ secret }}</code>
            <button class="primary" @click="copySecret">
              {{ copied ? t("tokens.copied") : t("tokens.copy") }}
            </button>
          </div>
        </div>

        <form class="token-form" @submit.prevent="generate">
          <input
            v-model="name"
            class="token-name"
            :placeholder="t('tokens.namePrompt')"
            :disabled="busy"
          />
          <input
            v-model="expiresDays"
            class="token-days"
            :placeholder="t('tokens.expiryPrompt')"
            inputmode="numeric"
            :disabled="busy"
          />
          <button class="primary" type="submit" :disabled="busy">
            {{ t("tokens.add") }}
          </button>
        </form>

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
          <button class="primary" @click="emit('close')">{{ t("tokens.close") }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
