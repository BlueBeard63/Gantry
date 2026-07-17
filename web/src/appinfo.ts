// The app's own identity, served by the built-in "gantry" service the
// Go side registers in gantry.Run: name, title and the version stamped
// from gantry.json by gantry dev/build.

import { useEffect, useState } from "react";
import { callGo } from "./socket";

export interface AppInfo {
  /** gantry.json "name" - the exe/module identity. */
  name: string;
  /** gantry.json "title" - the human-facing name. */
  title: string;
  /** gantry.json "version", stamped into the exe by gantry dev/build. */
  version: string;
}

let cached: AppInfo | null = null;
let pending: Promise<AppInfo> | null = null;

/** fetchAppInfo resolves the app's name/title/version once and caches
 * it - the values cannot change while the app runs. */
export function fetchAppInfo(): Promise<AppInfo> {
  if (cached) return Promise.resolve(cached);
  pending ??= callGo("gantry", "appInfo")
    .then((v) => (cached = v as AppInfo))
    .catch((err) => {
      pending = null; // retry on the next call
      throw err;
    });
  return pending;
}

/**
 * useAppInfo returns { name, title, version } - null until the first
 * fetch resolves. The obvious About-page one-liner:
 *
 *	const info = useAppInfo();
 *	return <span>v{info?.version}</span>;
 */
export function useAppInfo(): AppInfo | null {
  const [info, setInfo] = useState(cached);
  useEffect(() => {
    if (info) return;
    let alive = true;
    fetchAppInfo()
      .then((v) => {
        if (alive) setInfo(v);
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [info]);
  return info;
}
