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
      path: "/settings",
      name: "settings",
      component: () => import("./views/SettingsView.vue"),
      redirect: { name: "settings-appearance" },
      children: [
        {
          path: "appearance",
          name: "settings-appearance",
          component: () => import("./views/settings/AppearanceSection.vue"),
        },
        {
          path: "editor",
          name: "settings-editor",
          component: () => import("./views/settings/EditorSection.vue"),
        },
        {
          path: "ai",
          name: "settings-ai",
          component: () => import("./views/settings/AISection.vue"),
        },
        {
          path: "mail",
          name: "settings-mail",
          component: () => import("./views/settings/MailSection.vue"),
        },
        {
          path: "site",
          name: "settings-site",
          component: () => import("./views/settings/SiteSection.vue"),
        },
        {
          path: "storage",
          name: "settings-storage",
          component: () => import("./views/settings/StorageSection.vue"),
        },
        {
          path: "data",
          name: "settings-data",
          component: () => import("./views/settings/DataSection.vue"),
        },
        {
          path: "desktop",
          name: "settings-desktop",
          component: () => import("./views/settings/DesktopSection.vue"),
        },
        {
          path: "sync",
          name: "settings-sync",
          component: () => import("./views/settings/SyncSection.vue"),
        },
        {
          path: "users",
          name: "settings-users",
          component: () => import("./views/settings/UsersSection.vue"),
        },
        {
          path: "tokens",
          name: "settings-tokens",
          component: () => import("./views/settings/TokensSection.vue"),
        },
      ],
    },
    {
      path: "/capture",
      redirect: { name: "home" },
    },
    {
      path: "/login",
      name: "login",
      component: () => import("./views/LoginView.vue"),
      meta: { plain: true },
    },
    {
      path: "/register",
      name: "register",
      component: () => import("./views/RegisterView.vue"),
      meta: { plain: true, public: true },
    },
    {
      path: "/forgot-password",
      name: "forgot-password",
      component: () => import("./views/ForgotPasswordView.vue"),
      meta: { plain: true, public: true },
    },
    {
      path: "/reset-password",
      name: "reset-password",
      component: () => import("./views/ResetPasswordView.vue"),
      meta: { plain: true, public: true },
    },
    {
      path: "/accept-invite",
      name: "accept-invite",
      component: () => import("./views/AcceptInviteView.vue"),
      meta: { plain: true, public: true },
    },
    {
      path: "/verify-email",
      name: "verify-email",
      component: () => import("./views/VerifyEmailView.vue"),
      meta: { plain: true, public: true },
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
    if (to.name === "settings" || String(to.name ?? "").startsWith("settings-")) {
      return { name: "public-home" };
    }
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
