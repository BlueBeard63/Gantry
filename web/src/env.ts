// The app's mode and declared args, served by the built-in "gantry"
// service the Go side registers in gantry.Run. Mode is "development"
// under gantry dev and "production" in a built binary; args are the
// gantry.json "args" declarations resolved from their environment
// variables. Use them to gate pages and features.

import { useEffect, useState } from "react";
import { callGo } from "./socket";

export type AppMode = "development" | "production";

export interface AppEnv {
  /** "development" under gantry dev, "production" in a built binary. */
  mode: AppMode;
  /** Every declared arg (gantry.json "args"), resolved: env > default. */
  args: Record<string, string | number | boolean>;
}

let cached: AppEnv | null = null;
let pending: Promise<AppEnv> | null = null;

/** fetchEnv resolves the mode and declared args once and caches them -
 * the values cannot change while the app runs. */
export function fetchEnv(): Promise<AppEnv> {
  if (cached) return Promise.resolve(cached);
  pending ??= callGo("gantry", "env")
    .then((v) => (cached = v as AppEnv))
    .catch((err) => {
      pending = null; // retry on the next call
      throw err;
    });
  return pending;
}

/**
 * useEnv returns { mode, args } - null until the first fetch resolves.
 *
 *	const env = useEnv();
 *	if (env?.args["mock-data"]) ...
 */
export function useEnv(): AppEnv | null {
  const [env, setEnv] = useState(cached);
  useEffect(() => {
    if (env) return;
    let alive = true;
    fetchEnv()
      .then((v) => {
        if (alive) setEnv(v);
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [env]);
  return env;
}

/** useMode returns the app mode - null until the first fetch resolves. */
export function useMode(): AppMode | null {
  return useEnv()?.mode ?? null;
}

/**
 * useArg returns one declared arg's value - undefined until the first
 * fetch resolves (and for undeclared names).
 *
 *	const host = useArg<string>("api-host");
 */
export function useArg<T extends string | number | boolean = string | number | boolean>(name: string): T | undefined {
  return useEnv()?.args[name] as T | undefined;
}
