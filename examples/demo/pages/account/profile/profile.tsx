import { useState } from "react";
import { usePaired } from "gantry-web";

export default function Profile() {
  const { send } = usePaired(); // key injected: "pages/account/profile"
  const [name, setName] = useState("Jack");

  return (
    <div className="profile-page">
      <h2>Profile</h2>
      <p>A nested page: pages/account/profile serves /account/profile.</p>
      <label className="profile-field">
        Name
        <input value={name} onChange={(e) => setName(e.target.value)} />
      </label>
      <button onClick={() => send("rename", name)}>Rename</button>
    </div>
  );
}
