import { createApp } from "vue";
import { createPinia } from "pinia";
import { setUnauthorizedHandler } from "./api";
import App from "./App.vue";
import { router } from "./router";
import { useAuthStore } from "./stores/auth";
import { useSettingsStore } from "./stores/settings";
import "./styles.css";

const app = createApp(App);
app.use(createPinia());
app.use(router);

// Apply theme/font preferences before first paint.
useSettingsStore();

setUnauthorizedHandler(() => {
  const auth = useAuthStore();
  auth.clear();
  router.push({ name: "login", query: { redirect: router.currentRoute.value.fullPath } });
});

app.mount("#app");

if ("serviceWorker" in navigator && import.meta.env.PROD) {
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js").catch(() => undefined);
  });
}
