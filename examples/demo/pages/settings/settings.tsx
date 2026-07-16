import { useState } from "react";
import { usePaired } from "gantry-web";
import Example from "../../components/example/example";

export default function Settings() {
  const { send } = usePaired();
  const [name, setName] = useState("");

  return (
    <div className="settings-page">
      <h2>Settings</h2>
      <label className="settings-field">
        Display name
        <input value={name} onChange={(e) => setName(e.target.value)} />
      </label>
      <div className="settings-actions">
        <button onClick={() => send("save", { name })}>Save</button>
      </div>
      <Example />
    </div>
  );
}
