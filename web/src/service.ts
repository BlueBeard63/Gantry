import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { callGo, connect, GantryCallError } from "./socket";

export interface Service {
  /** call awaits a Go function registered on this service (ui.Calls). */
  call: <T = unknown>(name: string, payload?: unknown) => Promise<T>;
}

/** service returns a plain (non-hook) handle usable anywhere. */
export function service(name: string): Service {
  connect();
  return {
    call: <T,>(fn: string, payload?: unknown) => callGo(name, fn, payload) as Promise<T>,
  };
}

/**
 * useService is the hook form - the building block for app hooks like
 * useAuth():
 *
 *	function useAuth() {
 *	  const auth = useService("auth");
 *	  const me = useCall<User>("auth", "me");
 *	  return { ...me, login: (u: string, p: string) => auth.call("login", { u, p }) };
 *	}
 *
 * The Go side registers the functions once:
 *
 *	app.Service("auth", ui.Calls{ "me": ..., "login": ... })
 */
export function useService(name: string): Service {
  return useMemo(() => service(name), [name]);
}

export interface CallResult<T> {
  data: T | undefined;
  error: string | null;
  /** The gerr code of a failed call ("panic.call", "auth.expired", ...). */
  code: string | null;
  loading: boolean;
  /** reload re-runs the call. */
  reload: () => void;
}

/**
 * useCall runs a Go call on mount (and when key/name/payload change)
 * and hands back { data, error, loading, reload } - the fetch-shaped
 * companion to useService for read paths.
 */
export function useCall<T = unknown>(key: string, name: string, payload?: unknown): CallResult<T> {
  const [data, setData] = useState<T | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);
  const [code, setCode] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [tick, setTick] = useState(0);
  const alive = useRef(true);
  const payloadKey = JSON.stringify(payload ?? null);

  useEffect(() => {
    alive.current = true;
    setLoading(true);
    setError(null);
    setCode(null);
    callGo(key, name, payload)
      .then((v) => {
        if (alive.current) {
          setData(v as T);
          setLoading(false);
        }
      })
      .catch((e: Error) => {
        if (alive.current) {
          setError(e.message);
          if (e instanceof GantryCallError && e.code) setCode(e.code);
          setLoading(false);
        }
      });
    return () => {
      alive.current = false;
    };
    // payloadKey stands in for payload identity.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, name, payloadKey, tick]);

  const reload = useCallback(() => setTick((t) => t + 1), []);
  return { data, error, code, loading, reload };
}
