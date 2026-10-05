/// <reference types="vite/client" />

interface Window {
  /** Injected by the desktop shell before the app boots. */
  __OVERVIEW_DESKTOP__?: boolean;
}

declare module "turndown-plugin-gfm" {
  import type TurndownService from "turndown";
  export const gfm: TurndownService.Plugin;
  export const tables: TurndownService.Plugin;
  export const strikethrough: TurndownService.Plugin;
  export const taskListItems: TurndownService.Plugin;
}
