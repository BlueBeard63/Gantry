# Layouts

Layouts are the shared chrome around pages: navbars, sidebars, status bars - anything that should stay put while pages change. They live in the layouts/ directory and follow the same folder convention as pages and components.

```
layouts/
  main/
    main.tsx      the layout component (default export)
    main.css      optional styles, auto-imported
    main.go       optional Go half, paired under "layouts/main"
  compact/
    compact.tsx
```

## Writing a layout

A layout is a React component that renders its children where the page goes:

```tsx
// layouts/main/main.tsx
import { Link } from "gantry-web";
import type { ReactNode } from "react";

export default function Main({ children }: { children?: ReactNode }) {
  return (
    <div className="layout-main">
      <nav className="app-nav">
        <Link to="/">Home</Link>
        <Link to="/settings">Settings</Link>
      </nav>
      <main className="app-main">{children}</main>
    </div>
  );
}
```

The layout renders inside the app shell, below the TitleBar - window chrome is the [TitleBar](titlebar.md)'s job, page chrome is the layout's.

## Which pages get which layout

Layouts are addressed by short name: layouts/main -> "main". Each page declares its choice with an optional export; no export means the default:

```tsx
// nothing exported        -> "main", if a layout named main exists
export const layout = "compact";           // use layouts/compact instead
export const layout = ["main", "compact"]; // nest: <Main><Compact><Page/></Compact></Main>
export const layout = false;               // raw page, no layout
export const layout = true;                // force "main" on a chromeless page
```

Rules worth knowing:

- **"main" is the default name.** Have one layout, call it main, and every normal page picks it up with zero exports.
- **Chromeless pages** (`export const chrome = false` - widgets, popups) skip layouts by default; they are their own little surfaces. Opt back in with a name or `true`.
- **Arrays nest outermost-first**: `["main", "compact"]` puts the compact layout inside the main one - section-level chrome inside app-level chrome.
- **An unknown name logs a console warning and is skipped** - a typo never blanks the page.

## Active-aware navigation

The nav links inside a layout are the routing primitives - `Link` (with its active `data-active`/`aria-current` state), `useRoute`, `isActive`, and `ExternalLink` for links that leave the app. They are the same in a layout as anywhere else, so the full story lives on the [Routing](routing.md) page:

```tsx
import { Link, ExternalLink } from "gantry-web";

<Link to="/settings">Settings</Link>
<Link to="/docs" matchPrefix>Docs</Link>            // active on /docs/*
<ExternalLink href="https://example.com">Site</ExternalLink>
```

## Styling a layout

Same rules as everywhere (see [Styling](styling.md)): the colocated main.css auto-imports, scope selectors under a class your layout renders (`.layout-main`), and use the `--gantry-*` variables so the layout follows the theme. Layout css loads after the root index.css and before page css.

## Advanced: a layout with a Go half

A layout folder can hold a .go file like any pair - useful when the shared chrome shows live data (a status bar with a sync indicator, a sidebar with counts). Export a `Component` under the layouts key and it registers automatically like every other pair:

```go
// layouts/main/main.go
package main_layout // note: cannot be "package main" - pick a distinct name

var Component = ui.Component{
    Key: "layouts/main",
    On:  ui.Handlers{ /* events from the layout */ },
}
```

```tsx
// layouts/main/main.tsx
const { state } = usePaired(); // key injected: "layouts/main"
```

Then push into it from anywhere: `app.Push("layouts/main", "state", data)`. See [Pairs](pairs.md) for the full data-flow picture.

One Go naming caveat: a folder named `main` would collide with Go's `package main`. Either name the package differently (as above - Go package names need not match folders) or skip the .go half for the main layout and put live data in a component inside it.
