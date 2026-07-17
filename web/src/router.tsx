// Tiny pathname router shared by createApp, Link and useRoute. No
// dependency, history-API based: navigate() pushes, back/forward work,
// and every subscriber re-renders on change.

import {
  useSyncExternalStore,
  type AnchorHTMLAttributes,
  type ReactNode,
} from "react";
import { getShell } from "./bridge";

// Subscriptions go through a window-scoped event, NOT a module-local
// set: if bundling ever instantiates this module twice (it happened -
// Vite served excluded node_modules source under ?v=hash and bare URLs
// at once), a local set splits the subscribers and navigation silently
// stops re-rendering half the app. The window survives any number of
// module copies; the route itself already lives in location.pathname.
const NAV_EVENT = "gantry:navigate";

function notify() {
  window.dispatchEvent(new Event(NAV_EVENT));
}

if (typeof window !== "undefined") {
  window.addEventListener("popstate", notify);
}

/** navigate switches pages without a full reload. */
export function navigate(path: string): void {
  if (location.pathname !== path) {
    history.pushState(null, "", path);
  }
  notify();
}

/** redirect replaces the current URL (no history entry) - used when a
 * route no longer exists and the app falls back to another page. */
export function redirect(path: string): void {
  if (location.pathname !== path) {
    history.replaceState(null, "", path);
  }
  notify();
}

/** goBack navigates back through the page history (popstate re-renders). */
export function goBack(): void {
  history.back();
}

/** goForward navigates forward through the page history. */
export function goForward(): void {
  history.forward();
}

function subscribe(fn: () => void): () => void {
  window.addEventListener(NAV_EVENT, fn);
  return () => window.removeEventListener(NAV_EVENT, fn);
}

/** useRoute returns the current pathname; re-renders on navigation. */
export function useRoute(): string {
  return useSyncExternalStore(subscribe, () => location.pathname);
}

/** isActive: exact match, or prefix match when exact is false. */
export function isActive(path: string, to: string, exact = true): boolean {
  const clean = path !== "/" && path.endsWith("/") ? path.slice(0, -1) : path;
  if (exact || to === "/") return clean === to;
  return clean === to || clean.startsWith(to + "/");
}

export interface LinkProps extends AnchorHTMLAttributes<HTMLAnchorElement> {
  /** Route to navigate to, e.g. "/settings". */
  to: string;
  /** Extra class applied while the link's route is current. */
  activeClassName?: string;
  /** Match route prefixes too ("/docs" active on "/docs/intro"). */
  matchPrefix?: boolean;
  children?: ReactNode;
}

/**
 * Link navigates between pages and knows when it is the current one:
 * it sets data-active="true"/"false" (style with
 * [data-active="true"] in css, or data-[active=true]: variants in
 * Tailwind), aria-current="page", and appends activeClassName when
 * given.
 */
export interface ExternalLinkProps extends AnchorHTMLAttributes<HTMLAnchorElement> {
  /** The external URL to open. */
  href: string;
  prefix?: string;
  children?: ReactNode;
}

/**
 * ExternalLink opens a URL in the user's DEFAULT BROWSER instead of
 * navigating the app's webview, and (in the native window) renders
 * without an href so no URL-preview bubble ever appears. In a plain
 * browser tab it falls back to a normal new-tab link.
 */
export function ExternalLink({ href, prefix, children, onClick, ...rest }: ExternalLinkProps) {
  const shell = getShell(prefix);
  if (!shell.available) {
    return (
      <a href={href} target="_blank" rel="noreferrer" onClick={onClick} {...rest}>
        {children}
      </a>
    );
  }
  return (
    <a
      role="link"
      tabIndex={0}
      onClick={(e) => {
        onClick?.(e);
        if (!e.defaultPrevented) shell.openExternal(href);
      }}
      onKeyDown={(e) => {
        if (e.key === "Enter") shell.openExternal(href);
      }}
      {...rest}
    >
      {children}
    </a>
  );
}

export function Link({ to, activeClassName, matchPrefix, className, children, onClick, ...rest }: LinkProps) {
  const path = useRoute();
  const active = isActive(path, to, !matchPrefix);
  const cls = [className, active ? activeClassName : undefined].filter(Boolean).join(" ");
  // Deliberately NO href: navigation is client-side anyway, and an
  // href makes the webview show the URL preview bubble in the bottom
  // corner on hover - a browser artifact that looks wrong in a
  // desktop app. role/tabIndex/keyboard keep it accessible.
  return (
    <a
      role="link"
      tabIndex={0}
      className={cls || undefined}
      data-active={active ? "true" : "false"}
      aria-current={active ? "page" : undefined}
      onClick={(e) => {
        onClick?.(e);
        if (e.defaultPrevented || e.button !== 0) return;
        navigate(to);
      }}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          navigate(to);
        }
      }}
      {...rest}
    >
      {children}
    </a>
  );
}
