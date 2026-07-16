// The native bridge: typed access to the window.<prefix>* functions the
// Go appshell binds. In a plain browser tab none of them exist, so
// every method is safe to call anywhere - it just does nothing outside
// the native window - and `available` tells you which world you are in.

export interface ShellCaps {
  minimize: boolean;
  maximize: boolean;
  close: boolean;
  alwaysOnTop: boolean;
}

export interface ShellBridge {
  /** True when running inside a Gantry native window. */
  available: boolean;
  close(): void;
  minimize(): void;
  /** Start the native window drag loop (call on mousedown). */
  drag(): void;
  /** System notification sound + taskbar flash. */
  attention(): void;
  maximize(): void;
  restore(): void;
  isMaximized(): Promise<boolean>;
  /** Which window buttons the Go side enabled (all false in a browser). */
  caps(): Promise<ShellCaps>;
  /** Widgets and popups: show or hide the native window. */
  setVisible(show: boolean): void;
  /** Widgets: resize the native window in place. */
  resize(width: number, height: number): void;
  setAlwaysOnTop(on: boolean): void;
  /** Open a URL in the user's default browser (never in the app). */
  openExternal(url: string): void;
}

type AnyFn = (...args: unknown[]) => unknown;

function fn(prefix: string, name: string): AnyFn | undefined {
  const w = window as unknown as Record<string, unknown>;
  const f = w[prefix + name];
  return typeof f === "function" ? (f as AnyFn) : undefined;
}

const NO_CAPS: ShellCaps = {
  minimize: false,
  maximize: false,
  close: false,
  alwaysOnTop: false,
};

/**
 * getShell returns the bridge for the given binding prefix. The prefix
 * must match the Go side's BindingPrefix; both default to "gantry".
 */
export function getShell(prefix = "gantry"): ShellBridge {
  const call = (name: string, ...args: unknown[]) => {
    const f = fn(prefix, name);
    if (f) void f(...args);
  };
  return {
    // Minimize is bound on every main window unless disabled, and Close
    // exists on every window kind, so either marks a native host.
    available: !!(fn(prefix, "Minimize") ?? fn(prefix, "Close")),
    close: () => call("Close"),
    minimize: () => call("Minimize"),
    drag: () => call("Drag"),
    attention: () => call("Attention"),
    maximize: () => call("Maximize"),
    restore: () => call("Restore"),
    isMaximized: async () => {
      const f = fn(prefix, "IsMaximized");
      return f ? ((await f()) as boolean) : false;
    },
    caps: async () => {
      const f = fn(prefix, "Caps");
      return f ? ((await f()) as ShellCaps) : NO_CAPS;
    },
    setVisible: (show) => call("Visible", show),
    resize: (w, h) => call("Resize", w, h),
    setAlwaysOnTop: (on) => call("SetAlwaysOnTop", on),
    openExternal: (url) => call("OpenExternal", url),
  };
}
