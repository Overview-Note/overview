<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api } from "../../api";
import { t } from "../../i18n";
import { useAuthStore } from "../../stores/auth";

const auth = useAuthStore();
const isAdmin = computed(() => auth.user?.role === "admin");

const aiEnabled = ref(false);
const aiModel = ref("");
const aiBaseUrl = ref("");
const aiApiKey = ref("");
const aiHasKey = ref(false);
const aiSaving = ref(false);
const aiSaved = ref(false);

async function loadAI() {
  try {
    const status = await api.aiStatus();
    aiEnabled.value = status.enabled;
    aiModel.value = status.model ?? "";
  } catch {
    aiEnabled.value = false;
  }
  if (isAdmin.value) {
    try {
      const cfg = await api.aiSettings();
      aiBaseUrl.value = cfg.baseUrl;
      aiModel.value = cfg.model;
      aiHasKey.value = cfg.hasKey;
    } catch {
      /* ignore */
    }
  }
}

async function saveAI() {
  aiSaving.value = true;
  aiSaved.value = false;
  try {
    const cfg = await api.saveAISettings({
      baseUrl: aiBaseUrl.value,
      apiKey: aiApiKey.value,
      model: aiModel.value,
    });
    aiHasKey.value = cfg.hasKey;
    aiApiKey.value = "";
    aiSaved.value = true;
    await loadAI();
  } finally {
    aiSaving.value = false;
  }
}

onMounted(loadAI);
</script>

<template>
  <div class="settings-page">
    <h2>{{ t("ai.title") }}</h2>
    <section class="settings-card">
      <div class="settings-row">
        <span class="settings-label">{{ t("settings.aiStatus") }}</span>
        <span :class="aiEnabled ? 'status-ok' : 'status-off'">
          {{ aiEnabled ? t("settings.aiEnabled") : t("settings.aiDisabled") }}
        </span>
      </div>

      <template v-if="isAdmin">
        <label class="settings-field">
          <span>{{ t("settings.aiBaseUrl") }}</span>
          <input v-model="aiBaseUrl" placeholder="https://api.openai.com/v1" />
        </label>
        <label class="settings-field">
          <span>{{ t("settings.aiApiKey") }}</span>
          <input
            v-model="aiApiKey"
            type="password"
            :placeholder="aiHasKey ? t('settings.aiKeySet') : 'sk-...'"
          />
        </label>
        <label class="settings-field">
          <span>{{ t("settings.aiModelField") }}</span>
          <input v-model="aiModel" placeholder="gpt-4o-mini" />
        </label>
        <p class="settings-hint">{{ t("settings.aiHint") }}</p>
        <div class="settings-save">
          <button class="primary" :disabled="aiSaving" @click="saveAI">
            {{ aiSaving ? t("settings.saving") : t("settings.save") }}
          </button>
          <span v-if="aiSaved" class="status-ok">{{ t("settings.saved") }}</span>
        </div>
      </template>
      <p v-else class="settings-hint">{{ t("settings.aiAdminOnly") }}</p>
    </section>
  </div>
</template>
