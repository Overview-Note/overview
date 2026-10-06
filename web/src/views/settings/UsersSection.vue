<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ApiError, api, type User } from "../../api";
import { t } from "../../i18n";
import { useAuthStore } from "../../stores/auth";
import { useDialogStore } from "../../stores/dialog";

const auth = useAuthStore();
const dialogs = useDialogStore();
const isAdmin = computed(() => auth.isAdmin);

const users = ref<User[]>([]);
const error = ref("");
const ok = ref("");
const mailEnabled = ref(false);
const loading = ref(false);

const inviteEmail = ref("");
const inviteRole = ref<"member" | "admin">("member");
const inviteError = ref("");
const inviteOk = ref("");
const inviting = ref(false);

async function load() {
  error.value = "";
  loading.value = true;
  try {
    const [list, mail] = await Promise.all([api.listUsers(), api.mailSettings()]);
    users.value = list;
    mailEnabled.value = mail.enabled;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  if (isAdmin.value) void load();
});

async function invite() {
  inviteError.value = "";
  inviteOk.value = "";
  const email = inviteEmail.value.trim();
  if (!email) {
    inviteError.value = t("users.emailRequired");
    return;
  }
  inviting.value = true;
  try {
    await api.inviteUser(email, inviteRole.value);
    inviteEmail.value = "";
    inviteOk.value = t("users.inviteDone");
    await load();
  } catch (e) {
    const message = (e as Error).message;
    inviteError.value = message
      ? `${t("users.inviteFailed")}：${message}`
      : t("users.inviteFailed");
  } finally {
    inviting.value = false;
  }
}

async function resend(user: User) {
  error.value = "";
  ok.value = "";
  try {
    await api.resendInvite(user.id);
    ok.value = t("users.resendDone");
    await load();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function addEmail(user: User) {
  const email = await dialogs.ask(t("users.addEmail"), "", {
    placeholder: t("users.emailPrompt"),
  });
  if (!email) return;
  error.value = "";
  ok.value = "";
  try {
    const { verificationSent } = await api.setUserEmail(user.id, email.trim());
    ok.value = verificationSent
      ? t("users.verificationSent")
      : t("users.emailSaved");
    await load();
  } catch (e) {
    if (e instanceof ApiError && e.status === 409) {
      error.value = t("users.emailExists");
    } else if (e instanceof ApiError && e.status === 400) {
      error.value = t("users.emailInvalid");
    } else {
      error.value = (e as Error).message;
    }
  }
}

async function sendVerification(user: User) {
  error.value = "";
  ok.value = "";
  try {
    await api.sendVerification(user.id);
    ok.value = t("users.verificationSent");
    await load();
  } catch (e) {
    if (e instanceof ApiError && e.status === 503) {
      error.value = t("users.mailRequired");
    } else {
      error.value = (e as Error).message;
    }
  }
}

async function removeUser(user: User) {
  const okConfirm = await dialogs.askConfirm(
    t("users.deleteTitle"),
    t("users.deleteMessage", { name: user.email || user.username }),
  );
  if (!okConfirm) return;
  error.value = "";
  ok.value = "";
  try {
    await api.deleteUser(user.id);
    await load();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function addUser() {
  const name = await dialogs.ask(t("users.newTitle"), "");
  if (!name) return;
  const password = await dialogs.ask(t("users.passwordTitle"), "");
  if (!password) return;
  error.value = "";
  ok.value = "";
  try {
    await api.createUser(name, password, "member");
    await load();
  } catch (e) {
    error.value = (e as Error).message;
  }
}
</script>

<template>
  <div class="settings-page">
    <h2>{{ t("users.title") }}</h2>
    <template v-if="isAdmin">
      <section class="settings-card">
        <h3 class="settings-section-title">{{ t("users.inviteTitle") }}</h3>
        <div class="invite-form">
          <label class="settings-field">
            <span>{{ t("users.email") }}</span>
            <input
              v-model="inviteEmail"
              type="email"
              autocomplete="off"
              placeholder="user@example.com"
              @keydown.enter.prevent="invite"
            />
          </label>
          <label class="settings-field">
            <span>{{ t("users.role") }}</span>
            <select v-model="inviteRole">
              <option value="member">{{ t("users.roleMember") }}</option>
              <option value="admin">{{ t("users.roleAdmin") }}</option>
            </select>
          </label>
          <button class="primary" :disabled="!mailEnabled || inviting" @click="invite">
            {{ inviting ? t("settings.saving") : t("users.invite") }}
          </button>
        </div>
        <p v-if="!mailEnabled" class="settings-hint">
          {{ t("users.mailRequired") }}
          <RouterLink class="auth-alt" :to="{ name: 'settings-mail' }">
            {{ t("mail.title") }}
          </RouterLink>
        </p>
        <p v-if="inviteError" class="auth-error">{{ inviteError }}</p>
        <p v-if="inviteOk" class="status-ok">{{ inviteOk }}</p>
      </section>

      <section class="settings-card">
        <div class="user-list">
          <div v-for="user in users" :key="user.id" class="user-row">
            <div class="user-meta">
              <span class="user-name">{{ user.email || user.username }}</span>
              <span class="user-badges">
                <span
                  class="badge"
                  :class="user.status === 'active' ? 'ok' : 'warn'"
                  :title="t('users.status')"
                >
                  {{
                    user.status === "invited"
                      ? t("users.statusInvited")
                      : t("users.statusActive")
                  }}
                </span>
                <span class="badge">
                  {{ user.role === "admin" ? t("users.admin") : t("users.member") }}
                </span>
                <span
                  v-if="user.email"
                  class="badge"
                  :class="user.emailVerified ? 'ok' : 'warn'"
                  :title="t('users.email')"
                >
                  {{
                    user.emailVerified
                      ? t("users.emailVerified")
                      : t("users.emailUnverified")
                  }}
                </span>
              </span>
            </div>
            <div class="user-actions">
              <button v-if="!user.email" class="small" @click="addEmail(user)">
                {{ t("users.addEmail") }}
              </button>
              <button
                v-else-if="!user.emailVerified && user.status === 'active'"
                class="small"
                @click="sendVerification(user)"
              >
                {{ t("users.sendVerification") }}
              </button>
              <button v-if="user.status === 'invited'" class="small" @click="resend(user)">
                {{ t("users.resend") }}
              </button>
              <button class="danger-text" @click="removeUser(user)">
                {{ t("users.delete") }}
              </button>
            </div>
          </div>
        </div>
        <p v-if="loading && users.length === 0" class="muted">{{ t("app.loading") }}</p>
        <p v-if="error" class="auth-error">{{ error }}</p>
        <p v-if="ok" class="status-ok">{{ ok }}</p>
      </section>

      <section class="settings-card">
        <details class="manual-create">
          <summary class="settings-section-title">{{ t("users.manualTitle") }}</summary>
          <div class="settings-save">
            <button class="primary" @click="addUser">{{ t("users.add") }}</button>
          </div>
        </details>
      </section>
    </template>
    <section v-else class="settings-card">
      <p class="settings-hint">{{ t("settings.adminOnly") }}</p>
    </section>
  </div>
</template>
