import { useCallback, useSyncExternalStore } from "react";
import { connect, getGoState, hasGoState, onGoState, setGoState } from "./socket";

/**
 * useGoState is useState whose value lives in Go. Declare the variable
 * on the Go side once:
 *
 *	volume := ui.NewState(app, "volume", 0.5)
 *
 * then use it anywhere in React exactly like useState:
 *
 *	const [volume, setVolume] = useGoState("volume", 0.5);
 *
 * Every component using the same key shares the value. setVolume
 * updates locally at once and writes through to Go; Go-side Set()
 * calls re-render every subscriber instantly. Until the first sync
 * arrives the hook returns `fallback`.
 */
export function useGoState<T>(key: string, fallback: T): [T, (v: T) => void] {
  const subscribe = useCallback((fn: () => void) => {
    connect();
    return onGoState(key, fn);
  }, [key]);
  const value = useSyncExternalStore(subscribe, () =>
    hasGoState(key) ? (getGoState(key) as T) : fallback,
  );
  const set = useCallback((v: T) => setGoState(key, v), [key]);
  return [value, set];
}
