<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import {
  api,
  type SyncConfigInput,
  type SyncConflict,
  type SyncDirection,
  type SyncStatus,
} from "../../api";
import { t } from "../../i18n";
import { useSiteStore } from "../../stores/site";

const site = useSiteStore();

const status = ref<SyncStatus | null>(null);
const conflicts = ref<SyncConflict[]>([]);
const loadError = ref("");

const serverURL = ref("");
const token = ref("");
const enabled = ref(false);
const direction = ref<SyncDirection>("both");
const intervalSec = ref(60);

const saving = ref(false);
const saved = ref(false);
const running = ref(false);
const resolving = ref("");
const error = ref("");

let timer: number | undefined;

const directions: { value: SyncDirection; labelKey: string }[] = [
  { value: "both", labelKey: "sync.directionBoth" },
  { value: "pull", labelKey: "sync.directionPull" },
  { value: "push", labelKey: "sync.directionPush" },
];

const statusKey = computed(() => {
  switch (status.value?.status) {
    case "syncing":
      return "sync.statusSyncing";
    case "error":
      return "sync.statusError";
    case "paused":
      return "sync.statusPaused";
    default:
      return "sync.statusIdle";
  }
});

const statusClass = computed(() => {
  if (!status.value) return "status-off";
  if (status.value.status === "error") return "auth-error";
  return status.value.connected ? "status-ok" : "status-off";
});

function applyStatus(s: SyncStatus) {
  status.value = s;
  serverURL.value = s.serverURL ?? "";
  enabled.value = s.enabled;
  direction.value = s.direction || "both";
  intervalSec.value = s.intervalSec || 60;
  conflicts.value = s.conflicts ?? [];
}

function formatTime(iso: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime()) || d.getFullYear() <= 1) return "—";
  return d.toLocaleString();
}

function isValidURL(value: string): boolean {
  if (!value) return true;
  if (value.startsWith("https://")) return true;
  try {
    const url = new URL(value);
    const host = url.hostname;
    return (
      url.protocol === "http:" &&
      (host === "127.0.0.1" || host === "localhost" || host === "::1" || host === "[::1]")
    );
  } catch {
    return false;
  }
}

async function load() {
  if (!site.desktop) return;
  loadError.value = "";
  try {
    applyStatus(await api.syncStatus());
  } catch (e) {
    loadError.value = (e as Error).message;
  }
}

async function refreshStatus() {
  if (!site.desktop) return;
  try {
    applyStatus(await api.syncStatus());
  } catch {
    /* keep the last known state */
  }
}

function sleep(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

async function save() {
  if (saving.value) return;
  saved.value = false;
  error.value = "";
  const url = serverURL.value.trim();
  if (!isValidURL(url)) {
    error.value = t("sync.invalidURL");
    return;
  }
  intervalSec.value = Math.min(300, Math.max(5, Number(intervalSec.value) || 60));
  saving.value = true;
  try {
    const cfg: SyncConfigInput = {
      serverURL: url,
      enabled: enabled.value,
      direction: direction.value,
      intervalSec: intervalSec.value,
    };
    if (token.value) cfg.token = token.value;
    applyStatus(await api.saveSync(cfg));
    token.value = "";
    saved.value = true;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    saving.value = false;
  }
}

async function run() {
  if (running.value) return;
  running.value = true;
  error.value = "";
  try {
    await api.runSync();
    await sleep(600);
    await refreshStatus();
    let polls = 0;
    while (status.value?.status === "syncing" && polls < 30) {
      await sleep(1000);
      await refreshStatus();
      polls += 1;
    }
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    running.value = false;
  }
}

async function resolve(c: SyncConflict, keep: "local" | "remote") {
  if (resolving.value) return;
  resolving.value = c.id;
  error.value = "";
  try {
    await api.resolveSyncConflict(c.id, keep);
    await refreshStatus();
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    resolving.value = "";
  }
}

function startPolling() {
  stopPolling();
  if (!site.desktop) return;
  timer = window.setInterval(() => {
    if (enabled.value) void refreshStatus();
  }, 5000);
}

function stopPolling() {
  if (timer !== undefined) {
    window.clearInterval(timer);
    timer = undefined;
  }
}

onMounted(async () => {
  await load();
  startPolling();
});

onBeforeUnmount(stopPolling);
</script>

<template>
  <div v-if="site.desktop" class="settings-page">
    <h2>{{ t("sync.title") }}</h2>

    <section class="settings-card">
      <label class="settings-field">
        <span>{{ t("sync.serverURL") }}</span>
        <input
          v-model="serverURL"
          placeholder="https://notes.example.com"
          autocomplete="off"
          spellcheck="false"
        />
      </label>
      <p class="settings-hint">{{ t("sync.serverURLHint") }}</p>

      <label class="settings-field">
        <span>{{ t("sync.token") }}</span>
        <input
          v-model="token"
          type="password"
          :placeholder="t('sync.tokenSet')"
          autocomplete="new-password"
        />
      </label>

      <div class="settings-row">
        <span class="settings-label">{{ t("sync.enabled") }}</span>
        <label class="switch">
          <input v-model="enabled" type="checkbox" />
          <span class="track"></span>
        </label>
      </div>

      <div class="settings-row">
        <span class="settings-label">{{ t("sync.direction") }}</span>
        <div class="segmented">
          <button
            v-for="d in directions"
            :key="d.value"
            :class="{ on: direction === d.value }"
            @click="direction = d.value"
          >
            {{ t(d.labelKey) }}
          </button>
        </div>
      </div>

      <label class="settings-field">
        <span>{{ t("sync.interval") }}</span>
        <input v-model.number="intervalSec" type="number" min="5" max="300" step="5" />
      </label>

      <p class="settings-hint">{{ t("sync.hint") }}</p>

      <div class="settings-save">
        <button class="primary" :disabled="saving" @click="save">
          {{ saving ? t("sync.saving") : t("sync.save") }}
        </button>
        <button :disabled="running" @click="run">
          {{ running ? t("sync.running") : t("sync.run") }}
        </button>
        <span v-if="saved" class="status-ok">{{ t("sync.saved") }}</span>
      </div>
      <p v-if="error" class="auth-error">{{ error }}</p>
      <p v-else-if="loadError" class="auth-error">{{ loadError }}</p>
    </section>

    <section class="settings-card">
      <h3 class="settings-section-title">{{ t("sync.status") }}</h3>
      <template v-if="status">
        <div class="sync-status">
          <div class="sync-status-item">
            <span class="sync-status-label">{{ t("sync.status") }}</span>
            <span :class="statusClass">
              {{ status.connected ? t("sync.connected") : t("sync.disconnected") }}
              · {{ t(statusKey) }}
            </span>
          </div>
          <div class="sync-status-item">
            <span class="sync-status-label">{{ t("sync.lastSync") }}</span>
            <span>{{ formatTime(status.lastSyncAt) }}</span>
          </div>
          <div class="sync-status-item">
            <span class="sync-status-label">{{ t("sync.progress") }}</span>
            <span>{{ status.progress.done }} / {{ status.progress.total }}</span>
          </div>
          <div class="sync-status-item">
            <span class="sync-status-label">{{ t("sync.conflicts") }}</span>
            <span>{{ conflicts.length }}</span>
          </div>
        </div>
        <p v-if="status.lastError" class="auth-error">
          {{ t("sync.lastError") }}: {{ status.lastError }}
        </p>
        <p v-if="!status.enabled" class="settings-hint">{{ t("sync.hint") }}</p>
      </template>
      <p v-else class="settings-hint">{{ t("sync.hint") }}</p>
    </section>

    <section class="settings-card">
      <h3 class="settings-section-title">{{ t("sync.conflicts") }}</h3>
      <p v-if="conflicts.length === 0" class="settings-hint">
        {{ t("sync.noConflicts") }}
      </p>
      <ul v-else class="sync-conflicts">
        <li v-for="c in conflicts" :key="c.id" class="sync-conflict">
          <div class="sync-conflict-info">
            <code class="settings-code">{{ c.originalPath }}</code>
            <span class="sync-conflict-arrow" aria-hidden="true">→</span>
            <code class="settings-code">{{ c.conflictPath }}</code>
            <span class="settings-hint">{{ formatTime(c.detectedAt) }}</span>
          </div>
          <div class="settings-save inline">
            <button :disabled="resolving === c.id" @click="resolve(c, 'local')">
              {{ t("sync.keepLocal") }}
            </button>
            <button :disabled="resolving === c.id" @click="resolve(c, 'remote')">
              {{ t("sync.keepRemote") }}
            </button>
          </div>
        </li>
      </ul>
    </section>
  </div>
</template>
