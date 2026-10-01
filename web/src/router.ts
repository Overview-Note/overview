import { createRouter, createWebHistory } from "vue-router";
import EmptyState from "./components/EmptyState.vue";
import EditorPane from "./components/EditorPane.vue";
import { useAuthStore } from "./stores/auth";
import LoginView from "./views/LoginView.vue";
import PublicHomeView from "./views/PublicHomeView.vue";
import PublicNoteView from "./views/PublicNoteView.vue";
import SetupView from "./views/SetupView.vue";
import TrashView from "./views/TrashView.vue";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", name: "home", component: EmptyState },
    {
      path: "/note/:path(.*)",
      name: "note",
      component: EditorPane,
      props: true,
    },
    { path: "/trash", name: "trash", component: TrashView },
    { path: "/login", name: "login", component: LoginView, meta: { plain: true } },
    { path: "/setup", name: "setup", component: SetupView, meta: { plain: true } },
    {
      path: "/public",
      name: "public-home",
      component: PublicHomeView,
      meta: { plain: true, public: true },
    },
    {
      path: "/public/:path(.*)",
      name: "public",
      component: PublicNoteView,
      props: true,
      meta: { plain: true, public: true },
    },
  ],
});

router.beforeEach(async (to) => {
  const auth = useAuthStore();
  if (!auth.loaded) {
    await auth.loadState().catch(() => undefined);
  }
  if (to.meta.public) return true;
  if (auth.mode !== "multi") return true;
  if (auth.needsSetup) {
    return to.name === "setup" ? true : { name: "setup" };
  }
  if (!auth.user) {
    return to.name === "login" ? true : { name: "login" };
  }
  if (to.name === "login" || to.name === "setup") {
    return { name: "home" };
  }
  return true;
});
