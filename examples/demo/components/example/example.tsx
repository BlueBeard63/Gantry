// A reusable paired component. Import it like any React component, or
// render it from a Tea View with ui.Custom("components/example", nil).
import { usePaired } from "gantry-web";

export default function Example() {
  const { send, state } = usePaired();
  return (
    <div className="example-card">
      <span>example component{state != null ? " - " + JSON.stringify(state) : ""}</span>
      <button onClick={() => send("ping", Date.now())}>ping Go</button>
    </div>
  );
}
