// Exercises args & modes (useEnv), loading states (Await + Skeleton
// over a slow Go call) and the crash pipeline: every button below
// triggers a different error kind so you can watch the built-in error
// UI, the terminal log and call("gantry","errors") react.
import { useState } from "react";
import { Await, Skeleton, useCall, useEnv, usePaired } from "gantry-web";
import { TeaView } from "gantry-web/tea";

function RenderBomb(): never {
  throw new Error("RenderBomb: deliberate render crash");
}

export default function Debug() {
  const { send, call } = usePaired();
  const env = useEnv();
  const users = useCall<string[]>("pages/debug", "slowUsers");
  const [explode, setExplode] = useState(false);

  return (
    <div style={{ padding: 16, display: "flex", flexDirection: "column", gap: 16 }}>
      <h2>Debug</h2>

      <section>
        <h3>Environment</h3>
        <pre>{env ? JSON.stringify(env, null, 2) : "loading..."}</pre>
      </section>

      <section>
        <h3>Loading states (slow Go call, reload to replay)</h3>
        <Await call={users} fallback={<Skeleton lines={3} />}>
          {(list) => (
            <ul>
              {list.map((u) => (
                <li key={u}>{u}</li>
              ))}
            </ul>
          )}
        </Await>
        <button onClick={users.reload}>Reload</button>
      </section>

      <section>
        <h3>Crash the app (on purpose)</h3>
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
          <button onClick={() => call("callBoom").catch(() => {})}>Go call panic</button>
          <button onClick={() => send("eventBoom")}>Go event panic</button>
          <button onClick={() => send("goroutineBoom")}>Go goroutine panic</button>
          <button
            onClick={() => {
              throw new Error("deliberate uncaught JS error");
            }}
          >
            JS error
          </button>
          <button onClick={() => void Promise.reject(new Error("deliberate unhandled rejection"))}>JS rejection</button>
          <button onClick={() => setExplode(true)}>React render crash</button>
        </div>
        {explode && <RenderBomb />}
      </section>

      <section>
        <h3>Notifications (mobile)</h3>
        <p>Posts a system notification on device; a logged no-op on desktop.</p>
        <button onClick={() => send("notify")}>Notify</button>
      </section>

      <section>
        <h3>Crash the Tea loop (on purpose)</h3>
        <p>
          The counter below lives in <code>debug.go</code>. Update and View panics are recovered - the counter keeps its
          last good value and the last good tree stays on screen.
        </p>
        <TeaView />
      </section>

      <section>
        <h3>Kill the process (on purpose)</h3>
        <p>
          A panic on a plain goroutine is uncatchable: the app dies, the trace lands in <code>crash.log</code>, and the
          next launch reports it as a "crashed last run" notice.
        </p>
        <button onClick={() => send("fatalBoom")}>Fatal crash (kills the app)</button>
      </section>
    </div>
  );
}
