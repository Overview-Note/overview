<script setup lang="ts">
import { ref, watch } from "vue";
import { api, type User } from "../api";
import { t } from "../i18n";
import { useDialogStore } from "../stores/dialog";

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ (e: "close"): void }>();

const dialogs = useDialogStore();
const users = ref<User[]>([]);
const error = ref("");

async function refresh() {
  error.value = "";
  try {
    users.value = await api.listUsers();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) void refresh();
  },
);

async function addUser() {
  const name = await dialogs.ask(t("users.newTitle"), "");
  if (!name) return;
  const password = await dialogs.ask(t("users.passwordTitle"), "");
  if (!password) return;
  try {
    await api.createUser(name, password, "member");
    await refresh();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function removeUser(user: User) {
  const ok = await dialogs.askConfirm(
    t("users.deleteTitle"),
    t("users.deleteMessage", { name: user.username }),
  );
  if (!ok) return;
  try {
    await api.deleteUser(user.id);
    await refresh();
  } catch (e) {
    error.value = (e as Error).message;
  }
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="overlay" @click.self="emit('close')">
      <div class="dialog dialog-wide">
        <h3>{{ t("users.title") }}</h3>
        <div class="user-list">
          <div v-for="user in users" :key="user.id" class="user-row">
            <span class="user-name">{{ user.username }}</span>
            <span class="badge">
              {{ user.role === "admin" ? t("users.admin") : t("users.member") }}
            </span>
            <button class="danger-text" @click="removeUser(user)">
              {{ t("users.delete") }}
            </button>
          </div>
        </div>
        <p v-if="error" class="auth-error">{{ error }}</p>
        <div class="dialog-actions">
          <button @click="addUser">{{ t("users.add") }}</button>
          <button class="primary" @click="emit('close')">{{ t("users.close") }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
