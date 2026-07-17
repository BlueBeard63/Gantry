import type { Plugin } from "vite";

export interface GantryPluginOptions {
  /** App root relative to the Vite root (default ".."). */
  appRoot?: string;
  /** Go server port for the /api and /gantry/ws proxies (default: gantry.json "port", else 8330). */
  goPort?: number;
}

export declare function gantry(opts?: GantryPluginOptions): Plugin;
export default gantry;
