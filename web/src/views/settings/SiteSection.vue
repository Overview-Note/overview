<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api } from "../../api";
import { t } from "../../i18n";
import { useAuthStore } from "../../stores/auth";

const auth = useAuthStore();
const isAdmin = computed(() => auth.user?.role === "admin");

const registrationEnabled = ref(false);
const loginHint = ref("");
const loginIcp = ref("");
const loginLinkText = ref("");
const loginLinkUrl = ref("");
const saving = ref(false);
const saved = ref(false);
const error = ref("");

async function loadSite() {
  if (!isAdmin.value) return;
  error.value = "";
  try {
    const cfg = await api.siteSettings();
    registrationEnabled.value = cfg.registrationEnabled;
    loginHint.value = cfg.loginHint;
    loginIcp.value = cfg.loginIcp;
    loginLinkText.value = cfg.loginLinkText;
    loginLinkUrl.value = cfg.loginLinkUrl;
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function saveSite() {
  saving.value = true;
  saved.value = false;
  error.value = "";
  try {
    const cfg = await api.saveSiteSettings({
      registrationEnabled: registrationEnabled.value,
      loginHint: loginHint.value,
      loginIcp: loginIcp.value,
      loginLinkText: loginLinkText.value,
      loginLinkUrl: loginLinkUrl.value,
    });
    registrationEnabled.value = cfg.registrationEnabled;
    loginHint.value = cfg.loginHint;
    loginIcp.value = cfg.loginIcp;
    loginLinkText.value = cfg.loginLinkText;
    loginLinkUrl.value = cfg.loginLinkUrl;
    saved.value = true;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    saving.value = false;
  }
}

onMounted(loadSite);
</script>

<template>
  <div class="settings-page">
    <h2>{{ t("site.title") }}</h2>
    <section class="settings-card">
      <template v-if="isAdmin">
        <div class="settings-row">
          <span class="settings-label">{{ t("site.registration") }}</span>
          <label class="switch">
            <input v-model="registrationEnabled" type="checkbox" />
            <span class="track"></span>
          </label>
        </div>
        <p class="settings-hint">{{ t("site.registrationHint") }}</p>

        <label class="settings-field">
          <span>{{ t("site.loginHint") }}</span>
          <textarea v-model="loginHint" rows="3"></textarea>
        </label>
        <label class="settings-field">
          <span>{{ t("site.loginIcp") }}</span>
          <input v-model="loginIcp" />
        </label>
        <label class="settings-field">
          <span>{{ t("site.loginLinkText") }}</span>
          <input v-model="loginLinkText" />
        </label>
        <label class="settings-field">
          <span>{{ t("site.loginLinkUrl") }}</span>
          <input v-model="loginLinkUrl" placeholder="https://example.com" />
        </label>

        <div class="settings-save">
          <button class="primary" :disabled="saving" @click="saveSite">
            {{ saving ? t("settings.saving") : t("settings.save") }}
          </button>
          <span v-if="saved" class="status-ok">{{ t("site.saved") }}</span>
        </div>
        <p v-if="error" class="auth-error">{{ error }}</p>
      </template>
      <p v-else class="settings-hint">{{ t("settings.adminOnly") }}</p>
    </section>
  </div>
</template>
