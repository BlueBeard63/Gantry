// Ambient types for the module the Gantry Vite plugin generates. This
// file keeps editors happy without the synthesized .gantry/ folder
// existing - include it via tsconfig "types": ["gantry-web/types"].
// The module's shape is GantryAppModule (exported by gantry-web); the
// synthesized .gantry/main.tsx passes the whole module to createApp.

declare module "virtual:gantry-app" {
  import type { FC, ReactNode } from "react";

  export type { GantryPage, GantryPageModule } from "gantry-web";

  export const pages: import("gantry-web").GantryPage[];
  export const components: Record<string, { default: FC }>;
  /** Layouts by short name: layouts/main/main.tsx -> "main". */
  export const layouts: Record<string, { default: FC<{ children?: ReactNode }> }>;
  /** The root app.tsx module (default export: CreateAppOptions), or null. */
  export const appConfig: { default: import("gantry-web").CreateAppOptions } | null;
  export const singlePage: boolean;
}
