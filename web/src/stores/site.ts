import { defineStore } from "pinia";
import { ref } from "vue";

export const useSiteStore = defineStore("site", () => {
  const render = ref(false);
  const siteTitle = ref("Overview");
  const loaded = ref(false);

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

  return { render, siteTitle, loaded, load };
});
