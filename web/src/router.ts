import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "./stores/auth";
import { useSiteStore } from "./stores/site";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/",
      name: "home",
      component: () => import("./components/EmptyState.vue"),
    },
    {
      path: "/note/:path(.*)",
      name: "note",
      component: () => import("./components/EditorPane.vue"),
      props: true,
    },
    {
      path: "/trash",
      name: "trash",
      component: () => import("./views/TrashView.vue"),
    },
    {
      path: "/capture",
      name: "capture",
      component: () => import("./views/CaptureView.vue"),
      meta: { plain: true },
    },
    {
      path: "/login",
      name: "login",
      component: () => import("./views/LoginView.vue"),
      meta: { plain: true },
    },
    {
      path: "/setup",
      name: "setup",
      component: () => import("./views/SetupView.vue"),
      meta: { plain: true },
    },
    {
      path: "/public",
      name: "public-home",
      component: () => import("./views/PublicHomeView.vue"),
      meta: { plain: true, public: true },
    },
    {
      path: "/public/:path(.*)",
      name: "public",
      component: () => import("./views/PublicNoteView.vue"),
      props: true,
      meta: { plain: true, public: true },
    },
  ],
});

router.beforeEach(async (to) => {
  const site = useSiteStore();
  if (!site.loaded) {
    await site.load();
  }
  // Documentation-site mode: everything is public and read-only.
  if (site.render) {
    if (to.name === "home") return { name: "public-home" };
    if (to.name === "note") return { name: "public", params: { path: to.params.path } };
    if (to.name === "capture") return { name: "public-home" };
    return true;
  }

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
