<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";

const auth = useAuthStore();
const router = useRouter();

const username = ref("");
const password = ref("");
const confirm = ref("");
const error = ref("");
const busy = ref(false);

async function submit() {
  error.value = "";
  if (password.value !== confirm.value) {
    error.value = "两次输入的密码不一致";
    return;
  }
  busy.value = true;
  try {
    await auth.setup(username.value, password.value);
    router.replace("/");
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="auth-page">
    <form class="auth-card" @submit.prevent="submit">
      <h1>Overview</h1>
      <p class="auth-sub">创建管理员账号以启用认证</p>
      <label>
        用户名
        <input v-model="username" autocomplete="username" autofocus />
      </label>
      <label>
        密码
        <input v-model="password" type="password" autocomplete="new-password" />
      </label>
      <label>
        确认密码
        <input v-model="confirm" type="password" autocomplete="new-password" />
      </label>
      <p v-if="error" class="auth-error">{{ error }}</p>
      <button class="primary" type="submit" :disabled="busy">
        {{ busy ? "创建中…" : "创建并登录" }}
      </button>
    </form>
  </div>
</template>
