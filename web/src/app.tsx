/// <reference path="../types/index.d.ts" />
import { Component, StrictMode, useEffect, type ErrorInfo as ReactErrorInfo, type FC, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import { TitleBar, type TitleBarProps } from "./TitleBar";
import { ResizeFrame } from "./ResizeFrame";
import { installZoomGuard } from "./zoom";
import { connect, ready, callGo } from "./socket";
import { useRoute, redirect } from "./router";
import { setRegistry, type ComponentRegistry, type TeaComponentProps } from "./tea/Runtime";
import { installErrorHandlers, reportError, useGantryErrors, dismissNotice, clearFatal, setDevMode, type ErrorHandlingOptions, type GantryErrorInfo } from "./errors";
import { ErrorScreen, type ErrorScreenProps } from "./ErrorScreen";
import { fetchEnv, useMode } from "./env";

/** The shape of a page module (a pages/<name>/<name>.tsx file). */
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

/**
 * The generated virtual:gantry-app module: the page/component/layout
 * registry the Vite plugin derives from the app's folders. The
 * synthesized .gantry/main.tsx imports it and hands it to createApp.
 * gantry-web itself must never import the virtual module: esbuild
 * cannot resolve virtual ids, so that import would force the package
 * out of dep prebundling - and serving it as raw node_modules source
 * lets Vite double-instantiate modules (?v=hash vs bare URLs), which
 * split the router's subscriber set and broke navigation.
 */
export interface GantryAppModule {
  pages: GantryPage[];
  components: Record<string, { default: FC }>;
  /** Layouts by short name: layouts/main/main.tsx -> "main". */
  layouts: Record<string, { default: FC<{ children?: ReactNode }> }>;
  /** The root app.tsx module (default export: CreateAppOptions), or null. */
  appConfig: { default: CreateAppOptions } | null;
  singlePage?: boolean;
}

// The registry handed to createApp; module-level so match/layoutsFor
// read it without threading props.
let reg: GantryAppModule;

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
  /**
   * Crash detection and interception - on by default. A React render
   * crash shows a full-screen error (full detail in development, a
   * friendly card in production) instead of a white screen; Go panics
   * and unhandled JS errors show dismissible notices in development.
   * Replace the UI (screen), intercept (onError), or turn it off.
   */
  errors?: ErrorHandlingOptions & { screen?: FC<ErrorScreenProps> };
}

function keyClass(key: string): string {
  return "gantry-" + key.replace(/[^a-zA-Z0-9]+/g, "-");
}

function routeOf(p: GantryPage): string {
  return p.mod.route ?? p.route;
}

function match(path: string): GantryPage | undefined {
  const clean = path !== "/" && path.endsWith("/") ? path.slice(0, -1) : path;
  return reg.pages.find((p) => routeOf(p) === clean) ?? reg.pages.find((p) => routeOf(p) === "/");
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
    names = reg.layouts.main ? ["main"] : [];
  } else {
    // Default: the "main" layout on normal pages; chromeless pages
    // (widgets, popups) are their own surfaces and skip it.
    names = chrome && reg.layouts.main ? ["main"] : [];
  }
  const out: FC<{ children?: ReactNode }>[] = [];
  for (const n of names) {
    const mod = reg.layouts[n];
    if (mod?.default) {
      out.push(mod.default);
    } else {
      console.warn(`gantry: page ${page.key} wants unknown layout "${n}" (have: ${Object.keys(reg.layouts).join(", ") || "none"})`);
    }
  }
  return out;
}

// ErrorBoundary catches render crashes below it. It wraps the page
// content only - ResizeFrame and TitleBar stay outside, so a crashed
// page keeps its drag and close controls.
class ErrorBoundary extends Component<{ children: ReactNode }, { crashed: boolean }> {
  state = { crashed: false };
  static getDerivedStateFromError() {
    return { crashed: true };
  }
  componentDidCatch(error: Error, info: ReactErrorInfo) {
    reportError({
      kind: "react-render",
      code: "react.render",
      message: error.message,
      stack: error.stack,
      componentStack: info.componentStack ?? undefined,
      time: new Date().toISOString(),
      origin: "js",
    });
  }
  render() {
    // The fatal overlay renders from the store (ErrorHost); the crashed
    // subtree just goes away.
    return this.state.crashed ? null : this.props.children;
  }
}

// ErrorHost renders the (default or custom) error UI from the store:
// the fatal overlay and the notice banners.
function ErrorHost({ options }: { options: CreateAppOptions }) {
  const { fatal, notices } = useGantryErrors();
  const mode = useMode();
  if (options.errors?.enabled === false) return null;
  const Screen = options.errors?.screen ?? ErrorScreen;
  return (
    <>
      {fatal && <Screen error={fatal} mode={mode} variant="fatal" onDismiss={clearFatal} />}
      {notices.map((e, i) => (
        // Newest first: the CSS shows only the first banner, dismissing
        // it reveals the next.
        <Screen key={i} error={e} mode={mode} variant="notice" onDismiss={() => dismissNotice(i)} />
      )).reverse()}
    </>
  );
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
  // The boundary wraps the page and layouts only: a render crash takes
  // the content down, never the window chrome - the error overlay
  // appears and the TitleBar keeps drag/close working.
  return (
    <div className="gantry-app">
      <ResizeFrame prefix={options.prefix} />
      {chrome && <TitleBar title={options.title} prefix={options.prefix} {...options.titleBar} />}
      <ErrorBoundary>{content}</ErrorBoundary>
      <ErrorHost options={options} />
    </div>
  );
}

/**
 * createApp boots a Gantry frontend: zoom guard, websocket, component
 * registry, TitleBar, the root layout (layout.tsx at the app root, if
 * present), and the page router (single-page mode when only pages/index
 * exists). The synthesized .gantry/main.tsx calls this with the
 * virtual:gantry-app module (`import * as app from "virtual:gantry-app"`);
 * an app only touches it to pass options.
 */
export function createApp(app: GantryAppModule, options: CreateAppOptions = {}): void {
  reg = app;
  // The optional root app.tsx wins over the synthesized defaults, so
  // apps customize everything without owning the entry file.
  if (reg.appConfig?.default) {
    options = { ...options, ...reg.appConfig.default };
  }
  installZoomGuard();
  installErrorHandlers(options.errors ?? {});
  connect(options.socketURL);
  // Resolve the mode for the error UI (dev shows full detail), and
  // surface a crash from the previous run if Go recorded one.
  if (options.errors?.enabled !== false) {
    fetchEnv()
      .then((env) => setDevMode(env.mode === "development"))
      .catch(() => {});
    callGo("gantry", "errors")
      .then((list) => {
        const errs = (list ?? []) as (Omit<GantryErrorInfo, "origin"> & { kind: string })[];
        const crash = errs.filter((e) => e.kind === "process-crash").at(-1);
        if (crash) reportError({ ...crash, origin: "go" });
      })
      .catch(() => {});
  }

  const registry: ComponentRegistry = {};
  for (const [key, mod] of Object.entries(reg.components)) {
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
