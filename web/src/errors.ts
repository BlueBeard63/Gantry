// The frontend half of Gantry's error pipeline. createApp installs the
// window-level handlers and the ErrorBoundary reports render crashes
// here; Go-side errors arrive over the websocket as {"t":"error"}
// frames. Every error lands in one store that drives the (default or
// custom) error UI, and JS-side errors are reported back to Go so the
// gantry dev terminal, the ring buffer and the app's OnError hook see
// them too.

import { useSyncExternalStore } from "react";
import { callGo, onServerError } from "./socket";

/** One breadcrumb in an error's trail (recorded by the Go server). */
export interface GantryCrumb {
  time: string;
  type: "navigate" | "event" | "call" | "state" | "custom";
  detail: string;
  ok: boolean;
}

/** One captured error, from either side of the app. */
export interface GantryErrorInfo {
  /** "react-render" | "js-error" | "js-rejection" | the Go kinds
   * ("call-panic", "process-crash", ...). */
  kind: string;
  /** gerr code: "panic.call", "js.error", ... */
  code?: string;
  /** Where it happened: "pages/index.increment", a component stack head. */
  source?: string;
  message: string;
  stack?: string;
  /** React component stack, for render crashes. */
  componentStack?: string;
  time: string;
  /** Active page when it fired. */
  page?: string;
  /** Recent user actions leading up to it, oldest first. */
  trail?: GantryCrumb[];
  /** Which side captured it. */
  origin: "go" | "js";
}

export interface ErrorHandlingOptions {
  /** false turns the whole frontend pipeline off (default true). */
  enabled?: boolean;
  /** Replace the built-in error screen (see ErrorScreenProps). */
  screen?: unknown; // typed as FC<ErrorScreenProps> in app.tsx to avoid a cycle
  /** Intercept every error; return false to suppress the default UI. */
  onError?: (e: GantryErrorInfo) => boolean | void;
  /** Report JS-side errors to Go (terminal log, ring buffer, OnError
   * hook). Default true. */
  reportToGo?: boolean;
  /** Show Go-side errors (panics) as dismissible notices: true, false,
   * or only in development mode (the default). */
  showGoErrors?: boolean | "development";
}

interface ErrorStore {
  /** A render crash: the page is down, the overlay owns the screen. */
  fatal: GantryErrorInfo | null;
  /** Non-fatal errors (Go panics, unhandled rejections): banners. */
  notices: GantryErrorInfo[];
}

let store: ErrorStore = { fatal: null, notices: [] };
const listeners = new Set<() => void>();
let opts: ErrorHandlingOptions = {};
let installed = false;
let devMode: boolean | null = null; // resolved from the env service

// StrictMode and error bubbling can fire the same error twice within a
// tick; dedupe by message+stack in a short window.
let lastSig = "";
let lastSigAt = 0;

function emit(): void {
  listeners.forEach((fn) => fn());
}

function setStore(next: ErrorStore): void {
  store = next;
  emit();
}

/** setDevMode feeds the resolved mode in (createApp does this); the
 * default UI shows full detail only in development. */
export function setDevMode(dev: boolean): void {
  devMode = dev;
  emit();
}

export function isDevMode(): boolean | null {
  return devMode;
}

function shouldShowGoError(): boolean {
  const pref = opts.showGoErrors ?? "development";
  if (pref === "development") return devMode !== false; // unknown mode: show
  return pref;
}

/**
 * reportError feeds one error into the pipeline: dedupe, the app's
 * onError hook, the store (driving the error UI), and - for JS-side
 * errors - a best-effort report to Go.
 */
export function reportError(e: GantryErrorInfo): void {
  if (opts.enabled === false) return;
  const sig = e.message + "\n" + (e.stack ?? "");
  const now = Date.now();
  if (sig === lastSig && now - lastSigAt < 500) return;
  lastSig = sig;
  lastSigAt = now;

  if (!e.page && e.origin === "js") e.page = location.pathname;
  if (!e.time) e.time = new Date().toISOString();

  if (e.origin === "js" && opts.reportToGo !== false) {
    // Best-effort: the socket may be down; the console always has it.
    callGo("gantry", "reportError", e).catch(() => {});
  }

  const suppress = opts.onError?.(e) === false;
  if (suppress) return;

  if (e.kind === "react-render") {
    setStore({ ...store, fatal: e });
    return;
  }
  if (e.origin === "go" && !shouldShowGoError()) {
    console.warn(`gantry: go error [${e.kind}] ${e.source ?? ""}: ${e.message}`);
    return;
  }
  if (e.origin === "js" && devMode === false) {
    // Production: non-fatal JS errors stay in the console.
    console.warn(`gantry: error [${e.kind}]: ${e.message}`);
    return;
  }
  setStore({ ...store, notices: [...store.notices, e] });
}

/** dismissNotice removes one banner (or all, with no index). */
export function dismissNotice(index?: number): void {
  if (index === undefined) {
    setStore({ ...store, notices: [] });
  } else {
    setStore({ ...store, notices: store.notices.filter((_, i) => i !== index) });
  }
}

/** clearFatal drops the fatal overlay (the Reload button uses a real
 * reload instead; this is for custom screens that can recover). */
export function clearFatal(): void {
  setStore({ ...store, fatal: null });
}

/** useGantryErrors subscribes the error UI to the store. */
export function useGantryErrors(): ErrorStore {
  return useSyncExternalStore(
    (fn) => {
      listeners.add(fn);
      return () => listeners.delete(fn);
    },
    () => store,
  );
}

/**
 * addBreadcrumb records an app-specific action in the Go-side error
 * trail ("sync started"), alongside the automatic navigate/event/call
 * crumbs. Fire-and-forget.
 */
export function addBreadcrumb(detail: string): void {
  callGo("gantry", "breadcrumb", { detail }).catch(() => {});
}

/**
 * installErrorHandlers wires the window-level capture: uncaught JS
 * errors and unhandled promise rejections. createApp calls this once.
 */
export function installErrorHandlers(options: ErrorHandlingOptions): void {
  opts = options;
  if (installed || options.enabled === false) return;
  installed = true;

  // Go-side errors (panics, last-run crashes) arrive as {"t":"error"}
  // frames.
  onServerError((p) => {
    const e = p as Omit<GantryErrorInfo, "origin">;
    if (e && e.message) reportError({ ...e, origin: "go" });
  });

  window.addEventListener("error", (ev) => {
    reportError({
      kind: "js-error",
      code: "js.error",
      message: ev.message || String(ev.error ?? "unknown error"),
      stack: ev.error instanceof Error ? ev.error.stack : undefined,
      source: ev.filename ? `${ev.filename}:${ev.lineno}` : undefined,
      time: new Date().toISOString(),
      origin: "js",
    });
  });
  window.addEventListener("unhandledrejection", (ev) => {
    const reason = ev.reason;
    reportError({
      kind: "js-rejection",
      code: "js.rejection",
      message: reason instanceof Error ? reason.message : String(reason),
      stack: reason instanceof Error ? reason.stack : undefined,
      time: new Date().toISOString(),
      origin: "js",
    });
  });
}
