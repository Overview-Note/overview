<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api } from "../../api";
import { t } from "../../i18n";
import { useAuthStore } from "../../stores/auth";

const auth = useAuthStore();
const isAdmin = computed(() => auth.isAdmin);

const endpoint = ref("");
const region = ref("us-east-1");
const accessKey = ref("");
const secretKey = ref("");
const bucket = ref("");
const useSSL = ref(true);
const publicURL = ref("");
const hasSecret = ref(false);
const enabled = ref(false);
const saving = ref(false);
const saved = ref(false);
const testing = ref(false);
const testResult = ref<{ ok: boolean; message: string } | null>(null);
const error = ref("");

async function loadStorage() {
  if (!isAdmin.value) return;
  error.value = "";
  try {
    const cfg = await api.storageSettings();
    apply(cfg);
  } catch (e) {
    error.value = (e as Error).message;
  }
}

function apply(cfg: Awaited<ReturnType<typeof api.storageSettings>>) {
  endpoint.value = cfg.endpoint;
  region.value = cfg.region;
  accessKey.value = cfg.accessKey;
  bucket.value = cfg.bucket;
  useSSL.value = cfg.useSSL;
  publicURL.value = cfg.publicURL;
  hasSecret.value = cfg.hasSecret;
  enabled.value = cfg.enabled;
}

async function saveStorage() {
  saving.value = true;
  saved.value = false;
  testResult.value = null;
  error.value = "";
  try {
    const cfg = await api.saveStorageSettings({
      endpoint: endpoint.value,
      region: region.value,
      accessKey: accessKey.value,
      secretKey: secretKey.value,
      bucket: bucket.value,
      useSSL: useSSL.value,
      publicURL: publicURL.value,
    });
    apply(cfg);
    secretKey.value = "";
    saved.value = true;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    saving.value = false;
  }
}

async function testStorage() {
  testing.value = true;
  testResult.value = null;
  error.value = "";
  try {
    testResult.value = await api.testStorage({
      endpoint: endpoint.value,
      region: region.value,
      accessKey: accessKey.value,
      secretKey: secretKey.value,
      bucket: bucket.value,
      useSSL: useSSL.value,
      publicURL: publicURL.value,
    });
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    testing.value = false;
  }
}

onMounted(loadStorage);
</script>

<template>
  <div class="settings-page">
    <h2>{{ t("storage.title") }}</h2>
    <section class="settings-card">
      <template v-if="isAdmin">
        <div class="settings-row">
          <span class="settings-label">{{ t("storage.status") }}</span>
          <span :class="enabled ? 'status-ok' : 'status-off'">
            {{ enabled ? t("storage.enabled") : t("storage.disabled") }}
          </span>
        </div>

        <label class="settings-field">
          <span>{{ t("storage.endpoint") }}</span>
          <input v-model="endpoint" placeholder="s3.amazonaws.com" />
        </label>
        <label class="settings-field">
          <span>{{ t("storage.region") }}</span>
          <input v-model="region" placeholder="us-east-1" />
        </label>
        <label class="settings-field">
          <span>{{ t("storage.accessKey") }}</span>
          <input v-model="accessKey" />
        </label>
        <label class="settings-field">
          <span>{{ t("storage.secretKey") }}</span>
          <input
            v-model="secretKey"
            type="password"
            :placeholder="hasSecret ? t('storage.secretSet') : ''"
          />
        </label>
        <label class="settings-field">
          <span>{{ t("storage.bucket") }}</span>
          <input v-model="bucket" placeholder="overview-assets" />
        </label>
        <div class="settings-row">
          <span class="settings-label">{{ t("storage.useSSL") }}</span>
          <label class="switch">
            <input v-model="useSSL" type="checkbox" />
            <span class="track"></span>
          </label>
        </div>
        <label class="settings-field">
          <span>{{ t("storage.publicURL") }}</span>
          <input v-model="publicURL" placeholder="https://cdn.example.com" />
        </label>

        <p class="settings-hint">{{ t("storage.hint") }}</p>
        <div class="settings-save">
          <button class="primary" :disabled="saving" @click="saveStorage">
            {{ saving ? t("settings.saving") : t("settings.save") }}
          </button>
          <button :disabled="testing" @click="testStorage">
            {{ testing ? t("storage.testing") : t("storage.test") }}
          </button>
          <span v-if="saved" class="status-ok">{{ t("settings.saved") }}</span>
        </div>
        <p v-if="testResult" :class="testResult.ok ? 'status-ok' : 'auth-error'">
          {{ testResult.ok ? t("storage.testOk") : t("storage.testFail", { message: testResult.message }) }}
        </p>
        <p v-if="error" class="auth-error">{{ error }}</p>
      </template>
      <p v-else class="settings-hint">{{ t("storage.adminOnly") }}</p>
    </section>
  </div>
</template>
