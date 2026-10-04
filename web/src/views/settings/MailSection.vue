<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api } from "../../api";
import { t } from "../../i18n";
import { useAuthStore } from "../../stores/auth";

const auth = useAuthStore();
const isAdmin = computed(() => auth.user?.role === "admin");

const host = ref("");
const port = ref(587);
const username = ref("");
const password = ref("");
const from = ref("");
const starttls = ref(true);
const hasPassword = ref(false);
const enabled = ref(false);
const saving = ref(false);
const saved = ref(false);
const error = ref("");

async function loadMail() {
  if (!isAdmin.value) return;
  error.value = "";
  try {
    const cfg = await api.mailSettings();
    host.value = cfg.host;
    port.value = cfg.port;
    username.value = cfg.username;
    from.value = cfg.from;
    starttls.value = cfg.starttls;
    hasPassword.value = cfg.hasPassword;
    enabled.value = cfg.enabled;
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function saveMail() {
  saving.value = true;
  saved.value = false;
  error.value = "";
  try {
    const cfg = await api.saveMailSettings({
      host: host.value,
      port: Number(port.value) || 0,
      username: username.value,
      password: password.value,
      from: from.value,
      starttls: starttls.value,
    });
    host.value = cfg.host;
    port.value = cfg.port;
    username.value = cfg.username;
    from.value = cfg.from;
    starttls.value = cfg.starttls;
    hasPassword.value = cfg.hasPassword;
    enabled.value = cfg.enabled;
    password.value = "";
    saved.value = true;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    saving.value = false;
  }
}

onMounted(loadMail);
</script>

<template>
  <div class="settings-page">
    <h2>{{ t("mail.title") }}</h2>
    <section class="settings-card">
      <template v-if="isAdmin">
        <div class="settings-row">
          <span class="settings-label">{{ t("mail.status") }}</span>
          <span :class="enabled ? 'status-ok' : 'status-off'">
            {{ enabled ? t("mail.enabled") : t("mail.disabled") }}
          </span>
        </div>

        <label class="settings-field">
          <span>{{ t("mail.host") }}</span>
          <input v-model="host" placeholder="smtp.example.com" />
        </label>
        <label class="settings-field">
          <span>{{ t("mail.port") }}</span>
          <input v-model.number="port" type="number" min="1" max="65535" placeholder="587" />
        </label>
        <label class="settings-field">
          <span>{{ t("mail.username") }}</span>
          <input v-model="username" placeholder="user@example.com" />
        </label>
        <label class="settings-field">
          <span>{{ t("mail.password") }}</span>
          <input
            v-model="password"
            type="password"
            :placeholder="hasPassword ? t('mail.passwordSet') : ''"
          />
        </label>
        <label class="settings-field">
          <span>{{ t("mail.from") }}</span>
          <input v-model="from" placeholder="noreply@example.com" />
        </label>
        <div class="settings-row">
          <span class="settings-label">{{ t("mail.starttls") }}</span>
          <label class="switch">
            <input v-model="starttls" type="checkbox" />
            <span class="track"></span>
          </label>
        </div>

        <p class="settings-hint">{{ t("mail.hint") }}</p>
        <div class="settings-save">
          <button class="primary" :disabled="saving" @click="saveMail">
            {{ saving ? t("settings.saving") : t("settings.save") }}
          </button>
          <span v-if="saved" class="status-ok">{{ t("settings.saved") }}</span>
        </div>
        <p v-if="error" class="auth-error">{{ error }}</p>
      </template>
      <p v-else class="settings-hint">{{ t("mail.adminOnly") }}</p>
    </section>
  </div>
</template>
