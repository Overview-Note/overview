import { createRouter, createWebHistory } from "vue-router";
import EmptyState from "./components/EmptyState.vue";
import EditorPane from "./components/EditorPane.vue";

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
  ],
});
