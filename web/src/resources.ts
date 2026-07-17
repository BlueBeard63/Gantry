// Embedded app resources. Files placed in the app's resources/ directory
// are embedded into the binary once and served by the Go side at
// /resources/<path> (in gantry dev, the Vite plugin serves the same
// folder live off disk). Reference them by URL:
//
//   import { resourceURL } from "gantry-web";
//   <img src={resourceURL("img/logo.png")} />
//   const cfg = await fetch(resourceURL("cfg.json")).then((r) => r.json());
//   // fonts: `url(${resourceURL("fonts/inter.woff2")})`
//
// The same file is reachable from the paired .go code via
// gantry.Resource("img/logo.png") / gantry.Resources().

/** URL of an embedded resource, e.g. resourceURL("img/logo.png") -> "/resources/img/logo.png". */
export function resourceURL(name: string): string {
  return "/resources/" + String(name).replace(/^\/+/, "");
}
