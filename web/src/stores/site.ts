import { defineStore } from "pinia";
import { ref } from "vue";

// detectDesktop reports whether the app is running inside the desktop shell.
// The shell injects window.__OVERVIEW_DESKTOP__; ?desktop=1 covers previews.
export function detectDesktop(): boolean {
  if (typeof window === "undefined") return false;
  if (window.__OVERVIEW_DESKTOP__ === true) return true;
  return new URLSearchParams(window.location.search).get("desktop") === "1";
}

export const useSiteStore = defineStore("site", () => {
  const render = ref(false);
  const siteTitle = ref("Overview");
  const loaded = ref(false);
  const desktop = ref(detectDesktop());

  async function load() {
    try {
      const res = await fetch("/api/v1/health");
      const data = (await res.json()) as {
        render?: boolean;
        siteTitle?: string;
      };
      render.value = data.render === true;
      if (data.siteTitle) siteTitle.value = data.siteTitle;
    } catch {
      render.value = false;
    } finally {
      loaded.value = true;
      document.title = siteTitle.value;
    }
  }

  return { render, siteTitle, loaded, desktop, load };
});
