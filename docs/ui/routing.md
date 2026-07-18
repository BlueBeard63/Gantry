# Routing

Routing in Gantry is plain pathname switching with no dependency: a [page](pages.md) answers to a URL, `navigate` moves between pages, and the router re-renders. This page covers how folders become routes and how to move around; dynamic segments (`[id]`, `[...slug]`) get their own [Dynamic routes](dynamic-routes.md) page.

## How folders become routes

A page's route is derived from its folder unless the Go half sets `Route` (or the tsx exports `route`). The mapping follows the folder path under pages/:

- pages/index -> `/`
- pages/settings -> `/settings`
- pages/account/settings -> `/account/settings` (nesting to any depth)
- an "index" leaf maps to its parent: pages/account/index -> `/account`

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

With a single page (only pages/index) the router disappears entirely.

## Navigating

`navigate("/settings")` pushes history and re-renders; the browser back/forward buttons work. From a Tea Model or any tsx that just needs to jump somewhere, call it directly:

```tsx
import { navigate } from "gantry-web";
navigate("/settings");
```

## Active-aware navigation

`Link` is the navigation primitive built for navbars and layouts. It renders an `<a>`, navigates client-side, and knows whether it points at the current page:

```tsx
<Link to="/settings">Settings</Link>
<Link to="/docs" matchPrefix>Docs</Link>                    // active on /docs/*
<Link to="/stats" activeClassName="lit">Stats</Link>        // extra class while active
```

While active a Link carries `data-active="true"` and `aria-current="page"`, so styling works three ways:

```css
/* plain css */
.app-nav a[data-active="true"] { background: var(--gantry-control-bg); }
```

```
tailwind:        data-[active=true]:bg-zinc-800
activeClassName: whatever class you like
```

For fully custom nav elements (buttons, tabs, tree items), the same information is available as hooks:

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

## Leaving the app: ExternalLink

For links that leave the app (docs, your repo, a website), use `ExternalLink` instead: it opens the URL in the user's default browser rather than navigating the app window, and shows no URL-preview bubble:

```tsx
<ExternalLink href="https://github.com/B-Commissions/Gantry">Source</ExternalLink>
```

Under the hood `ExternalLink` calls `shell.openExternal(url)` from [useShell()](../shell/window.md) - the same one-liner you can call directly (from a button handler, say) to open a URL in the default browser without ever navigating the app window.

## Related pages

- [Dynamic routes](dynamic-routes.md) - `[id]`/`[...slug]` folders, `useParams`, and the Go half.
- [Layouts](layouts.md) - shared chrome around pages, where `Link` usually lives.
