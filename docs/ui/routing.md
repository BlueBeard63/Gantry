# Routing

Routing in Gantry is plain pathname switching with no dependency: a [page](pages.md) answers to a URL, `navigate` moves between pages, and every subscriber re-renders. This page covers how folders become routes and how to move around; dynamic segments (`[id]`, `[...slug]`) get their own [Dynamic routes](dynamic-routes.md) page.

## How folders become routes

A page's route is derived from its folder unless the Go half sets `Route` (or the tsx exports `route`). The mapping follows the folder path under `pages/`:

- `pages/index` -> `/`
- `pages/settings` -> `/settings`
- `pages/account/settings` -> `/account/settings` (nesting to any depth)
- an `index` leaf maps to its parent: `pages/account/index` -> `/account`

A folder can both BE a page and CONTAIN pages:

```
pages/
  account/
    settings/
      settings.tsx, settings.go, settings.css   -> /account/settings
      profile/
        profile.tsx, profile.go                 -> /account/settings/profile
```

Override the derived route from either side when you need to - keep them matching if you set both:

```go
var Page = ui.Page{Key: "pages/settings", Route: "/preferences"}
```

```tsx
export const route = "/preferences";
```

The tsx `route` export wins over the Go-derived route on the frontend, so a tsx-only page (no `.go`) can still name its own path. With a single page (only `pages/index`) the router disappears entirely and the app runs in single-page mode.

## Navigating

`navigate("/settings")` pushes a history entry and re-renders; the browser back/forward buttons work. From a Tea Model or any tsx that just needs to jump somewhere, call it directly:

```tsx
import { navigate } from "gantry-web";
navigate("/settings");
```

Three siblings cover the rest of history navigation, all imported from `gantry-web`:

- `redirect("/")` - replaces the current URL instead of pushing, so it leaves no back-button entry. Gantry uses it internally to fall a stale URL back to the index.
- `goBack()` / `goForward()` - step through history, exactly like the browser buttons.

## Active-aware navigation

`Link` is the navigation primitive built for navbars and layouts. It renders an `<a>`, navigates client-side on click (or Enter/Space), and knows whether it points at the current page:

```tsx
<Link to="/settings">Settings</Link>
<Link to="/docs" matchPrefix>Docs</Link>                    // active on /docs and /docs/*
<Link to="/stats" activeClassName="lit">Stats</Link>        // extra class while active
```

- **`to`** - the route to navigate to.
- **`matchPrefix`** - when set, the link is active for the whole subtree (`/docs` lights up on `/docs/intro`), not just an exact match.
- **`activeClassName`** - an extra class appended only while the link is active.

While active a `Link` carries `data-active="true"` and `aria-current="page"` (inactive is `data-active="false"`), so styling works three ways:

```css
/* plain css */
.app-nav a[data-active="true"] { background: var(--gantry-control-bg); }
```

```
tailwind:        data-[active=true]:bg-zinc-800
activeClassName: whatever class you like
```

`Link` deliberately renders **no `href`**: navigation is client-side anyway, and an `href` makes the native webview pop a URL-preview bubble in the corner on hover - a browser artifact that looks wrong in a desktop app. `role="link"`, `tabIndex` and keyboard handling keep it accessible.

For fully custom nav elements (buttons, tabs, tree items), the same information is available as hooks:

```tsx
import { useRoute, isActive, navigate } from "gantry-web";

const path = useRoute(); // current pathname, re-renders on every navigation
<button
  data-active={isActive(path, "/stats")}
  onClick={() => navigate("/stats")}
>
  Stats
</button>
```

- **`useRoute()`** returns the current `location.pathname` and re-renders the component on navigation.
- **`isActive(path, to, exact = true)`** compares a pathname to a route. It defaults to an exact match (ignoring a trailing slash); pass `false` as the third argument for prefix matching - which is exactly what `Link`'s `matchPrefix` does under the hood.

## Leaving the app: ExternalLink

For links that leave the app (docs, your repo, a website), use `ExternalLink` instead: in the native window it opens the URL in the user's **default browser** rather than navigating the app window, and renders without an `href` so no URL-preview bubble ever appears. In a plain browser tab it falls back to a normal `target="_blank"` new-tab link.

```tsx
<ExternalLink href="https://github.com/BlueBeard63/Gantry">Source</ExternalLink>
```

Under the hood `ExternalLink` calls `shell.openExternal(url)` from [useShell()](../shell/window-chrome.md#the-useshell-surface) - the same one-liner you can call directly (from a button handler, say) to open a URL in the default browser without ever navigating the app window.

## When nothing matches

A URL that resolves to no page (a folder deleted during dev, a stale bookmark) falls back to the index page and `redirect`s the address to `/`, so `Link`s light up correctly. A *dynamic* match is never treated as a fallback - its URL is already canonical, so it is left exactly as-is. The full resolution order (exact static routes first, then dynamic ones most-specific first) is covered on [Dynamic routes](dynamic-routes.md).

## Related pages

- [Dynamic routes](dynamic-routes.md) - `[id]`/`[...slug]` folders, `useParams`, and the Go half.
- [Layouts](layouts.md) - shared chrome around pages, where `Link` usually lives.
