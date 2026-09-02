# Layouts

Layouts are the shared chrome around pages: navbars, sidebars, status bars - anything that should stay mounted while the page underneath changes. They live in the `layouts/` directory and follow the same folder convention as pages and components: a folder per layout, a `.tsx` default export, an optional colocated `.css`, and an optional paired `.go` half.

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

A layout is a React component that receives the page as `children` and renders it wherever the page belongs. The type is `FC<{ children?: ReactNode }>` - that is exactly the shape `createApp` resolves each layout name to:

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

A layout wraps the page host but sits *inside* the app scaffold and *below* the TitleBar: the render tree is `.gantry-app` -> `ResizeFrame` + [`TitleBar`](titlebar.md) -> layouts (outermost first) -> `.gantry-page` -> your page. Window chrome is the TitleBar's job; page chrome is the layout's. The page's own wrapper (`.gantry-page.gantry-<key>`) is nested inside whatever your layout renders, so your layout's markup is always the outer frame.

## Which pages get which layout

Layouts are addressed by the short name derived from the folder: `layouts/main/main.tsx` -> `"main"`, `layouts/compact/compact.tsx` -> `"compact"`. Each page declares its choice with an optional `layout` export on the page module (`GantryPageModule.layout?: boolean | string | string[]`); no export means the default:

```tsx
// nothing exported        -> "main", if a layout named main exists
export const layout = "compact";           // use layouts/compact instead
export const layout = ["main", "compact"]; // nest: <Main><Compact><Page/></Compact></Main>
export const layout = false;               // raw page, no layout
export const layout = true;                // force "main" on a chromeless page
```

The resolution runs in `createApp`'s `layoutsFor` and is exactly:

- **No export** resolves to `["main"]` when a `main` layout exists *and* the page is not chromeless; otherwise `[]`. So one layout named `main` covers every normal page with zero exports.
- **`false`** resolves to `[]` - a raw page with no layout at all.
- **A string** resolves to that single layout.
- **An array** nests outermost-first: `["main", "compact"]` renders `<Main><Compact><Page/></Compact></Main>`, so app-level chrome sits outside section-level chrome.
- **`true`** forces `["main"]` when a `main` layout exists (its purpose is to opt a chromeless page back into the default).
- **Chromeless pages** (`export const chrome = false` - widgets, popups, splash screens) skip the default layout: they are their own little surfaces. Opt back in with an explicit name, an array, or `true`.
- **An unknown name is skipped with a `console.warn`** listing the layouts that do exist, so a typo degrades to "no layout" and never blanks the page.

## Active-aware navigation

The nav links inside a layout are the routing primitives - `Link` (with its `data-active`/`aria-current` state and optional `matchPrefix`), `useRoute`, `isActive`, and `ExternalLink` for links that open in the user's default browser instead of navigating the webview. They behave identically inside a layout and anywhere else, so the full story lives on the [Routing](routing.md) page:

```tsx
import { Link, ExternalLink } from "gantry-web";

<Link to="/settings">Settings</Link>
<Link to="/docs" matchPrefix>Docs</Link>            // active on /docs and /docs/*
<ExternalLink href="https://example.com">Site</ExternalLink>
```

Because the layout stays mounted across navigation, an active link recomputes its state on every route change without the layout itself re-mounting - the routing store re-renders subscribers in place.

## Styling a layout

Same rules as everywhere (see [Styling](styling.md)): the colocated `main.css` auto-imports, so scope your selectors under a class your layout renders (`.layout-main`), and reach for the `--gantry-*` variables so the layout tracks the theme. There is no framework-injected class on the layout wrapper - the class is whatever your component puts on its own root element, so pick a stable one and scope under it.

## A layout with a Go half

A layout folder can hold a `.go` file like any other pair - useful when the shared chrome shows live data (a status bar with a sync indicator, a sidebar with unread counts). Register a `ui.Component` whose `Key` is the layout key, and it wires up like every other pair; the paired `.tsx` reads the same key through `usePaired()` (see [Pairs](pairs.md)):

```go
// layouts/main/main.go
package mainlayout // NOT "package main" - see the caveat below

import "github.com/BlueBeard63/Gantry/ui"

var Component = ui.Component{
    Key: "layouts/main",
    On:  ui.Handlers{ /* events the layout's tsx sends */ },
    Call: ui.Calls{ /* requests the layout's tsx awaits */ },
}
```

```tsx
// layouts/main/main.tsx
import { usePaired } from "gantry-web";
const { state, send } = usePaired(); // key injected by the Vite plugin: "layouts/main"
```

Push live data into it from anywhere - `Setup`, a service, a watcher - with the layout key:

```go
app.Push("layouts/main", "state", syncStatus) // arrives as usePaired().state in the tsx
```

One Go naming caveat: a folder named `main` would push the package toward Go's reserved `package main`. Go package names need not match their folder, so name the package something distinct (`mainlayout` above). If you would rather not, skip the `.go` half on the `main` layout and put the live data in a paired *component* rendered inside it instead.
