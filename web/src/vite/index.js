// The Gantry Vite plugin. Plain JavaScript on purpose: vite.config is
// executed by Node, which cannot load TypeScript out of node_modules.
//
// What it does:
//   - scans <appRoot>/pages/** and <appRoot>/components/** for paired
//     <name>/<name>.tsx files and generates the virtual:gantry-app
//     module (route table + component registry, keyed by folder path)
//   - auto-imports <appRoot>/index.css and every colocated same-name
//     .css (root css first, so per-pair css can override variables)
//   - injects the pairing key into bare usePaired() calls, derived from
//     the file's folder path, so the tsx never repeats it
//   - proxies /api and /gantry/ws to the Go server during gantry dev
//   - watches pages/ and components/ so adding or removing a pair
//     regenerates the app without restarting the dev server

import fs from "node:fs";
import path from "node:path";

const VIRTUAL_ID = "virtual:gantry-app";
const RESOLVED_ID = "\0" + VIRTUAL_ID;

/**
 * @param {{ appRoot?: string, goPort?: number }} [opts]
 * @returns {import("vite").Plugin}
 */
export function gantry(opts = {}) {
  /** @type {string} */
  let appRoot = "";
  let goPort = opts.goPort ?? 0;

  function scanKind(kind) {
    const dir = path.join(appRoot, kind);
    const out = [];
    if (!fs.existsSync(dir)) return out;
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      if (!entry.isDirectory()) continue;
      const name = entry.name;
      const tsx = path.join(dir, name, name + ".tsx");
      if (!fs.existsSync(tsx)) continue;
      const css = path.join(dir, name, name + ".css");
      out.push({
        key: kind + "/" + name,
        name,
        tsx: tsx.split(path.sep).join("/"),
        css: fs.existsSync(css) ? css.split(path.sep).join("/") : null,
      });
    }
    out.sort((a, b) => a.key.localeCompare(b.key));
    return out;
  }

  function generate() {
    const pages = scanKind("pages");
    const components = scanKind("components");
    const lines = [];

    const layouts = scanKind("layouts");

    const rootCss = path.join(appRoot, "index.css");
    if (fs.existsSync(rootCss)) {
      lines.push(`import ${JSON.stringify(rootCss.split(path.sep).join("/"))};`);
    }
    for (const item of [...layouts, ...pages, ...components]) {
      if (item.css) lines.push(`import ${JSON.stringify(item.css)};`);
    }

    layouts.forEach((l, i) => {
      lines.push(`import * as l${i} from ${JSON.stringify(l.tsx)};`);
    });

    pages.forEach((p, i) => {
      lines.push(`import * as p${i} from ${JSON.stringify(p.tsx)};`);
    });
    components.forEach((c, i) => {
      lines.push(`import * as c${i} from ${JSON.stringify(c.tsx)};`);
    });

    const pageEntries = pages.map((p, i) => {
      // The derived route only; an optional "export const route" on the
      // page module overrides it at runtime (createApp checks mod.route
      // there - referencing it here would make rollup warn on every
      // page that does not export it).
      const route = p.name === "index" ? "/" : "/" + p.name;
      return `{ key: ${JSON.stringify(p.key)}, route: ${JSON.stringify(route)}, mod: p${i} }`;
    });
    lines.push(`export const pages = [${pageEntries.join(", ")}];`);

    const compEntries = components.map((c, i) => `${JSON.stringify(c.key)}: c${i}`);
    lines.push(`export const components = { ${compEntries.join(", ")} };`);

    // Layouts are keyed by their short name: layouts/main -> "main",
    // which is what pages reference in "export const layout".
    const layoutEntries = layouts.map((l, i) => `${JSON.stringify(l.name)}: l${i}`);
    lines.push(`export const layouts = { ${layoutEntries.join(", ")} };`);
    lines.push(`export const singlePage = ${pages.length <= 1};`);
    return lines.join("\n");
  }

  /** Derive the pairing key when id is a paired tsx/css file. */
  function keyFor(id) {
    const file = id.split("?")[0].split(path.sep).join("/");
    const root = appRoot.split(path.sep).join("/");
    if (!file.startsWith(root + "/")) return null;
    const rel = file.slice(root.length + 1);
    const m = rel.match(/^(pages|components|layouts)\/([^/]+)\/([^/]+)\.tsx$/);
    if (!m || m[2] !== m[3]) return null;
    return m[1] + "/" + m[2];
  }

  return {
    name: "gantry",

    config(userConfig, { command }) {
      const cfgRoot = userConfig.root
        ? path.resolve(userConfig.root)
        : process.cwd();
      appRoot = path.resolve(cfgRoot, opts.appRoot ?? "..");

      if (!goPort) {
        try {
          const cfg = JSON.parse(fs.readFileSync(path.join(appRoot, "gantry.json"), "utf8"));
          goPort = cfg.port ?? 8330;
        } catch {
          goPort = 8330;
        }
      }
      const target = "http://127.0.0.1:" + goPort;

      return {
        // gantry-web ships TypeScript source through an npm file: link;
        // excluding it from prebundling keeps it in the normal
        // transform pipeline (plugin-react handles its tsx), and
        // deduping react avoids a second copy through the symlink.
        optimizeDeps: { exclude: ["gantry-web"] },
        // dedupe makes imports inside the symlinked gantry-web source
        // resolve from the APP's node_modules (the package itself has
        // none) and guarantees a single React instance.
        resolve: { dedupe: ["react", "react-dom", "lucide-react"] },
        server:
          command === "serve"
            ? {
                proxy: {
                  "/api": { target },
                  "/gantry/ws": { target, ws: true },
                },
              }
            : undefined,
      };
    },

    resolveId(id) {
      if (id === VIRTUAL_ID) return RESOLVED_ID;
      return null;
    },

    load(id) {
      if (id === RESOLVED_ID) return generate();
      return null;
    },

    transform(code, id) {
      const key = keyFor(id);
      if (key && code.includes("usePaired()")) {
        return code.replaceAll("usePaired()", `usePaired(${JSON.stringify(key)})`);
      }
      return null;
    },

    configureServer(server) {
      server.watcher.add(appRoot);
      const refresh = (file) => {
        const f = file.split(path.sep).join("/");
        const root = appRoot.split(path.sep).join("/");
        if (!f.startsWith(root)) return;
        if (
          !/\/(pages|components|layouts)\/[^/]+\/[^/]+\.(tsx|css)$/.test(f) &&
          !f.endsWith("/index.css")
        ) {
          return;
        }
        const mod = server.moduleGraph.getModuleById(RESOLVED_ID);
        if (mod) server.moduleGraph.invalidateModule(mod);
        server.ws.send({ type: "full-reload" });
      };
      server.watcher.on("add", refresh);
      server.watcher.on("unlink", refresh);
    },
  };
}

export default gantry;
