/// <reference path="../types/index.d.ts" />
import { StrictMode, useEffect, type FC, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import { pages, components as pairedComponents, layouts, appConfig, type GantryPage } from "virtual:gantry-app";
import { TitleBar, type TitleBarProps } from "./TitleBar";
import { ResizeFrame } from "./ResizeFrame";
import { installZoomGuard } from "./zoom";
import { connect, ready } from "./socket";
import { useRoute, redirect } from "./router";
import { setRegistry, type ComponentRegistry, type TeaComponentProps } from "./tea/Runtime";
import "./styles.css";

export interface CreateAppOptions {
  /** Title shown in the TitleBar (default: none). */
  title?: ReactNode;
  /**
   * TitleBar customization: title placement (titleAlign), your own
   * controls in the left/right slots (back/forward buttons, menus),
   * heights and reserves. The usual home for this is the optional
   * app.tsx at the app root, default-exporting CreateAppOptions.
   */
  titleBar?: Partial<TitleBarProps>;
  /** Extra Tea components beyond the paired components/ folders. */
  components?: ComponentRegistry;
  /** Binding prefix when the Go side changed it from "gantry". */
  prefix?: string;
  /** Websocket URL override (default ws://<host>/gantry/ws). */
  socketURL?: string;
  /** Hide the TitleBar on every page (pages can also export chrome = false). */
  chrome?: boolean;
}

function keyClass(key: string): string {
  return "gantry-" + key.replace(/[^a-zA-Z0-9]+/g, "-");
}

function routeOf(p: GantryPage): string {
  return p.mod.route ?? p.route;
}

function match(path: string): GantryPage | undefined {
  const clean = path !== "/" && path.endsWith("/") ? path.slice(0, -1) : path;
  return pages.find((p) => routeOf(p) === clean) ?? pages.find((p) => routeOf(p) === "/");
}

function PageHost({ page }: { page: GantryPage }) {
  useEffect(() => {
    ready(page.key);
  }, [page.key]);
  const Page = page.mod.default;
  return (
    <div className={"gantry-page " + keyClass(page.key)}>
      <Page />
    </div>
  );
}

// layoutsFor resolves a page's layout selection to component functions,
// outermost first.
//
//   (nothing)            -> "main" if it exists (chromeless pages skip)
//   layout = false       -> none
//   layout = "compact"   -> that one
//   layout = ["main","compact"] -> nested: <Main><Compact><Page/>...
function layoutsFor(page: GantryPage, chrome: boolean): FC<{ children?: ReactNode }>[] {
  const sel = page.mod.layout;
  let names: string[];
  if (sel === false) {
    names = [];
  } else if (typeof sel === "string") {
    names = [sel];
  } else if (Array.isArray(sel)) {
    names = sel;
  } else if (sel === true) {
    names = layouts.main ? ["main"] : [];
  } else {
    // Default: the "main" layout on normal pages; chromeless pages
    // (widgets, popups) are their own surfaces and skip it.
    names = chrome && layouts.main ? ["main"] : [];
  }
  const out: FC<{ children?: ReactNode }>[] = [];
  for (const n of names) {
    const mod = layouts[n];
    if (mod?.default) {
      out.push(mod.default);
    } else {
      console.warn(`gantry: page ${page.key} wants unknown layout "${n}" (have: ${Object.keys(layouts).join(", ") || "none"})`);
    }
  }
  return out;
}

function AppRoot({ options }: { options: CreateAppOptions }) {
  const path = useRoute();
  const page = match(path);
  // A URL whose page no longer exists (deleted during dev, stale
  // bookmark) falls back to the index page - normalize the address so
  // Links light up correctly.
  const target = page ? routeOf(page) : null;
  const clean = path !== "/" && path.endsWith("/") ? path.slice(0, -1) : path;
  useEffect(() => {
    if (target !== null && target !== clean) {
      redirect(target);
    }
  }, [target, clean]);
  if (!page) {
    return <div className="gantry-app">No pages found - add pages/index/index.tsx</div>;
  }
  const chrome = options.chrome !== false && page.mod.chrome !== false;
  let content = <PageHost page={page} />;
  for (const Layout of layoutsFor(page, chrome).reverse()) {
    content = <Layout>{content}</Layout>;
  }
  return (
    <div className="gantry-app">
      <ResizeFrame prefix={options.prefix} />
      {chrome && <TitleBar title={options.title} prefix={options.prefix} {...options.titleBar} />}
      {content}
    </div>
  );
}

/**
 * createApp boots a Gantry frontend: zoom guard, websocket, component
 * registry, TitleBar, the root layout (layout.tsx at the app root, if
 * present), and the page router (single-page mode when only pages/index
 * exists). The synthesized .gantry/main.tsx calls this; an app only
 * touches it to pass options.
 */
export function createApp(options: CreateAppOptions = {}): void {
  // The optional root app.tsx wins over the synthesized defaults, so
  // apps customize everything without owning the entry file.
  if (appConfig?.default) {
    options = { ...options, ...appConfig.default };
  }
  installZoomGuard();
  connect(options.socketURL);

  const registry: ComponentRegistry = {};
  for (const [key, mod] of Object.entries(pairedComponents)) {
    if (typeof mod.default === "function") {
      registry[key] = mod.default as FC<TeaComponentProps>;
    }
  }
  Object.assign(registry, options.components);
  setRegistry(registry);

  const container = document.getElementById("root");
  if (!container) throw new Error("gantry: no #root element");
  createRoot(container).render(
    <StrictMode>
      <AppRoot options={options} />
    </StrictMode>,
  );
}
