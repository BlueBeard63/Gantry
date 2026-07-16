// Ambient types for the module the Gantry Vite plugin generates. This
// file keeps editors happy without the synthesized .gantry/ folder
// existing - include it via tsconfig "types": ["gantry-web/types"].

declare module "virtual:gantry-app" {
  import type { FC, ReactNode } from "react";

  export interface GantryPageModule {
    default: FC;
    /** export const chrome = false to hide the TitleBar on this page. */
    chrome?: boolean;
    /**
     * Which layouts/ wrap this page: a name ("compact"), several nested
     * outermost-first (["main", "compact"]), false for none, true to
     * force "main" even on a chromeless page. Default: "main" if it
     * exists (chromeless pages default to none).
     */
    layout?: boolean | string | string[];
    /** export const route = "/custom" to override the derived route. */
    route?: string;
  }

  export interface GantryPage {
    key: string;
    route: string;
    mod: GantryPageModule;
  }

  export const pages: GantryPage[];
  export const components: Record<string, { default: FC }>;
  /** Layouts by short name: layouts/main/main.tsx -> "main". */
  export const layouts: Record<string, { default: FC<{ children?: ReactNode }> }>;
  /** The root app.tsx module (default export: CreateAppOptions), or null. */
  export const appConfig: { default: import("gantry-web").CreateAppOptions } | null;
  export const singlePage: boolean;
}
