import { useCallback, useEffect, useMemo, useState } from "react";
import { callGo, connect, onPush, sendPaired } from "./socket";

export interface Paired {
  /** send fires a named event on this file's .go half (ui.Handlers). */
  send: (event: string, payload?: unknown) => void;
  /** call awaits a function on the .go half (ui.Calls). */
  call: <T = unknown>(name: string, payload?: unknown) => Promise<T>;
  /** on subscribes to pushes from Go (App.Push); returns unsubscribe. */
  on: (event: string, fn: (payload: unknown) => void) => () => void;
  /**
   * state is the latest payload Go pushed with the event name "state" -
   * the simple way to mirror Go-side data into the component.
   */
  state: unknown;
}

/**
 * usePaired is the channel between a .tsx file and the .go file sitting
 * next to it. Inside pages/ and components/ call it with no argument -
 * the Gantry Vite plugin fills the key in from the folder path at build
 * time. Anywhere else, pass the key explicitly: usePaired("pages/index").
 */
export function usePaired(key?: string): Paired {
  if (!key) {
    throw new Error(
      "usePaired(): no key. Inside pages/ or components/ the Gantry Vite " +
        "plugin injects it - make sure the file is under the app root and " +
        "the gantry() plugin is in vite.config. Elsewhere pass the key " +
        'explicitly, e.g. usePaired("pages/index").',
    );
  }
  const k = key;
  const [state, setState] = useState<unknown>(undefined);

  useEffect(() => {
    connect();
    return onPush(k, (event, payload) => {
      if (event === "state") setState(payload);
    });
  }, [k]);

  const send = useCallback((event: string, payload?: unknown) => sendPaired(k, event, payload), [k]);
  const call = useCallback(
    <T,>(name: string, payload?: unknown) => callGo(k, name, payload) as Promise<T>,
    [k],
  );
  const on = useCallback(
    (event: string, fn: (payload: unknown) => void) =>
      onPush(k, (e, payload) => {
        if (e === event) fn(payload);
      }),
    [k],
  );

  return useMemo(() => ({ send, call, on, state }), [send, call, on, state]);
}
