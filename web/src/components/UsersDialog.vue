<script setup lang="ts">
import { ref, watch } from "vue";
import { api, type User } from "../api";
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
  const name = await dialogs.ask("新建用户", "");
  if (!name) return;
  const password = await dialogs.ask("设置密码（至少 6 位）", "");
  if (!password) return;
  try {
    await api.createUser(name, password, "member");
    await refresh();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function removeUser(user: User) {
  const ok = await dialogs.askConfirm("删除用户", `确定删除「${user.username}」？`);
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
        <h3>用户管理</h3>
        <div class="user-list">
          <div v-for="user in users" :key="user.id" class="user-row">
            <span class="user-name">{{ user.username }}</span>
            <span class="badge">{{ user.role === "admin" ? "管理员" : "成员" }}</span>
            <button class="danger-text" @click="removeUser(user)">删除</button>
          </div>
        </div>
        <p v-if="error" class="auth-error">{{ error }}</p>
        <div class="dialog-actions">
          <button @click="addUser">＋ 新建用户</button>
          <button class="primary" @click="emit('close')">关闭</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
