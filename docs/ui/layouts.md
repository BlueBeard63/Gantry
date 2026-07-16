# Layouts

Layouts are shared chrome around pages: navbars, sidebars, status
bars - anything that should stay put while pages change. They live in
the layouts/ directory and follow the same folder convention as pages
and components.

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

A layout is a React component that renders its children where the page
goes:

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

The layout renders inside the app shell, below the TitleBar - window
chrome is the TitleBar's job (see [The TitleBar](titlebar.md)), page
chrome is the layout's.

## Which pages get which layout

Layouts are addressed by short name: layouts/main -> "main". Each page
declares its choice with an optional export; no export means the
default:

```tsx
// nothing exported        -> "main", if a layout named main exists
export const layout = "compact";           // use layouts/compact instead
export const layout = ["main", "compact"]; // nest: <Main><Compact><Page/></Compact></Main>
export const layout = false;               // raw page, no layout
export const layout = true;               // force "main" on a chromeless page
```

Rules worth knowing:

- "main" is the default name. Have one layout and call it main, and
  every normal page picks it up with zero exports.
- Chromeless pages (export const chrome = false - widgets, popups)
  skip layouts by default; they are their own little surfaces. Opt
  back in with a name or true.
- Arrays nest outermost-first: ["main", "compact"] puts the compact
  layout inside the main one. Use it for section-level chrome inside
  app-level chrome.
- An unknown name logs a console warning and is skipped - a typo never
  blanks the page.

## Active-aware navigation

Link is the navigation primitive built for layouts. It renders an <a>,
navigates client-side, and knows whether it points at the current page:

```tsx
<Link to="/settings">Settings</Link>
<Link to="/docs" matchPrefix>Docs</Link>                    // active on /docs/*
<Link to="/stats" activeClassName="lit">Stats</Link>        // extra class while active
```

While active a Link carries data-active="true" and aria-current="page",
so styling works three ways:

```css
/* plain css */
.app-nav a[data-active="true"] { background: var(--gantry-control-bg); }
```

```
tailwind: data-[active=true]:bg-zinc-800
activeClassName: whatever class you like
```

For links that leave the app (docs, your repo, a website), use
ExternalLink instead: it opens the URL in the user's default browser
rather than navigating the app window, and shows no URL-preview
bubble:

```tsx
<ExternalLink href="https://github.com/B-Commissions/Gantry">Source</ExternalLink>
```

For fully custom nav elements (buttons, tabs, tree items), the same
information is available as hooks:

```tsx
import { useRoute, isActive, navigate } from "gantry-web";

const path = useRoute(); // current pathname, re-renders on navigation
<button
  data-active={isActive(path, "/stats")}
  onClick={() => navigate("/stats")}
>
  Stats
</button>
```

## Styling a layout

Same rules as everywhere (see [Styling](styling.md)): the colocated
main.css auto-imports, scope selectors under a class your layout
renders (.layout-main), and use the --gantry-* variables so the layout
follows the theme. Layout css loads after the root index.css and before
page css.

## Layouts with a Go half

A layout folder can hold a .go file like any pair - useful when the
shared chrome shows live data (a status bar with a sync indicator, a
sidebar with counts). Export a Component under the layouts key and it
registers automatically like every other pair:

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

Then push into it from anywhere: app.Push("layouts/main", "state", data).

One Go naming caveat: a folder named main would collide with Go's
"package main". Either name the package differently (as above - Go
package names do not have to match folders) or skip the .go half for
the main layout and put live data in a component inside it.
