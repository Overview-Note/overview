import { createApp } from "vue";
import { createPinia } from "pinia";
import { setUnauthorizedHandler } from "./api";
import App from "./App.vue";
import { router } from "./router";
import { useAuthStore } from "./stores/auth";
import "./styles.css";

const app = createApp(App);
app.use(createPinia());
app.use(router);

setUnauthorizedHandler(() => {
  const auth = useAuthStore();
  auth.clear();
  router.push({ name: "login", query: { redirect: router.currentRoute.value.fullPath } });
});

app.mount("#app");
