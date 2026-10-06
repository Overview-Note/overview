import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { api, type User } from "../api";

export const useAuthStore = defineStore("auth", () => {
  const mode = ref<"none" | "multi">("none");
  const needsSetup = ref(false);
  const user = ref<User | null>(null);
  const mailEnabled = ref(false);
  const registrationEnabled = ref(false);
  const loginHint = ref("");
  const loginIcp = ref("");
  const loginLink = ref<{ text: string; url: string } | null>(null);
  const loaded = ref(false);

  const isAdmin = computed(
    () => mode.value === "none" || user.value?.role === "admin",
  );

  async function loadState() {
    const state = await api.authState();
    mode.value = state.mode;
    needsSetup.value = state.needsSetup;
    mailEnabled.value = state.mailEnabled;
    registrationEnabled.value = state.registrationEnabled ?? false;
    loginHint.value = state.loginHint ?? "";
    loginIcp.value = state.loginIcp ?? "";
    loginLink.value = state.loginLink ?? null;
    user.value = state.user ?? null;
    loaded.value = true;
  }

  async function setup(username: string, password: string) {
    user.value = await api.setup(username, password);
    needsSetup.value = false;
  }

  async function login(username: string, password: string) {
    user.value = await api.login(username, password);
  }

  async function logout() {
    await api.logout();
    user.value = null;
  }

  function clear() {
    user.value = null;
  }

  return {
    mode,
    needsSetup,
    user,
    mailEnabled,
    registrationEnabled,
    loginHint,
    loginIcp,
    loginLink,
    loaded,
    isAdmin,
    loadState,
    setup,
    login,
    logout,
    clear,
  };
});
