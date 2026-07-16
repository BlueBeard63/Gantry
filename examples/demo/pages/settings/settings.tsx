import { useState } from "react";
import { usePaired, useGoState } from "gantry-web";
import Example from "../../components/example/example";

export default function Settings() {
  const { send } = usePaired();
  const [name, setName] = useState("");
  // useGoState: a useState whose value lives in Go (declared with
  // ui.NewState in main.go). Both sides see every change instantly -
  // watch the gantry dev terminal while dragging the slider.
  const [volume, setVolume] = useGoState("volume", 0.5);

  return (
    <div className="settings-page">
      <h2>Settings</h2>
      <label className="settings-field">
        Display name
        <input value={name} onChange={(e) => setName(e.target.value)} />
      </label>
      <label className="settings-field">
        Volume (shared with Go): {Math.round(volume * 100)}%
        <input
          type="range"
          min={0}
          max={1}
          step={0.01}
          value={volume}
          onChange={(e) => setVolume(Number(e.target.value))}
        />
      </label>
      <div className="settings-actions">
        <button onClick={() => send("save", { name })}>Save</button>
      </div>
      <Example />
    </div>
  );
}
