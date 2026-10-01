<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";

const auth = useAuthStore();
const router = useRouter();
const route = useRoute();

const username = ref("");
const password = ref("");
const error = ref("");
const busy = ref(false);

async function submit() {
  error.value = "";
  busy.value = true;
  try {
    await auth.login(username.value, password.value);
    const redirect = (route.query.redirect as string) || "/";
    router.replace(redirect);
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
      <p class="auth-sub">登录到你的知识库</p>
      <label>
        用户名
        <input v-model="username" autocomplete="username" autofocus />
      </label>
      <label>
        密码
        <input v-model="password" type="password" autocomplete="current-password" />
      </label>
      <p v-if="error" class="auth-error">{{ error }}</p>
      <button class="primary" type="submit" :disabled="busy">
        {{ busy ? "登录中…" : "登录" }}
      </button>
    </form>
  </div>
</template>
