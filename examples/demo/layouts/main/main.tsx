// The "main" layout: shared chrome (navbar, sidebars, status bars)
// around every normal page. Pages pick layouts by name - export const
// layout = "other", or nest several with ["main", "other"] - and this
// one is the default. Add more as layouts/<name>/<name>.tsx.
import { Link } from "gantry-web";
import type { ReactNode } from "react";

export default function Main({ children }: { children?: ReactNode }) {
  return (
    <div className="layout-main">
      <nav className="app-nav">
        <Link to="/">Home</Link>
        <Link to="/settings">Settings</Link>
        <Link to="/account/profile">Profile</Link>
        <Link to="/examples/page1/1" matchPrefix>Dynamic</Link>
        <Link to="/files/docs/intro" matchPrefix>Catch-all</Link>
      </nav>
      <main className="app-main">{children}</main>
    </div>
  );
}
