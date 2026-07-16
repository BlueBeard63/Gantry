import { useEffect, useMemo, useState } from "react";
import { getShell, type ShellBridge, type ShellCaps } from "./bridge";

/** useShell memoizes the native bridge for components. */
export function useShell(prefix = "gantry"): ShellBridge {
  return useMemo(() => getShell(prefix), [prefix]);
}

/**
 * useShellCaps resolves which window buttons the Go window enabled.
 * Returns null while resolving (render nothing yet to avoid a button
 * flicker), then the caps - all false in a plain browser.
 */
export function useShellCaps(prefix = "gantry"): ShellCaps | null {
  const shell = useShell(prefix);
  const [caps, setCaps] = useState<ShellCaps | null>(null);
  useEffect(() => {
    let alive = true;
    if (!shell.available) {
      setCaps({ minimize: false, maximize: false, close: false, alwaysOnTop: false });
      return;
    }
    void shell.caps().then((c) => {
      if (alive) setCaps(c);
    });
    return () => {
      alive = false;
    };
  }, [shell]);
  return caps;
}
