# Pages

A page is a routable [pair](pairs.md): a .tsx/.go folder that the router can navigate to by URL. Everything about how the two halves talk - keys, `usePaired`, pushes, registration - is the shared [pairing model](pairs.md); this page is what makes a pair a *page*.

## The Go half

```go
var Page = ui.Page{
    Key:   "pages/settings",
    Route: "/settings",       // optional; derived from Key when empty
    Model: func() ui.Model { return model{} }, // optional Tea model
    On: ui.Handlers{          // optional plain handlers
        "save": func(p json.RawMessage) { /* ... */ },
    },
    Call: ui.Calls{           // optional awaited calls
        "load": func(p json.RawMessage) (any, error) { /* ... */ return data, nil },
    },
}
```

- `Key` - the folder path, the one string that ties the halves together (see [Pairs](pairs.md)).
- `Route` - the URL this page answers to; leave it empty to let Gantry derive it from the folder (see [Routing](routing.md)).
- `Model` - an optional Tea model to run the whole UI as a Go state machine (see [The Tea model](tea.md)).
- `On` - fire-and-forget [paired handlers](pairs.md).
- `Call` - awaited calls that return an answer (see [Calls and services](calls-and-services.md)).

A page needs a Model, handlers, calls, or none of them: a purely static page needs no .go logic at all (though the .go file must still exist if main.go registers it). A folder can both BE a page and CONTAIN pages - nesting works to any depth. See [Routing](routing.md) for how folders become URLs.

## The tsx half

The tsx half's default export is the page component. Three optional named exports tune it:

- `export const chrome = false` hides the [TitleBar](titlebar.md) on this page (widgets, popups).
- `export const route = "/x"` overrides the route (keep it matching the Go side if both are set - see [Routing](routing.md)).
- `export const layout = "..."` picks the shared chrome that wraps the page (see [Layouts](layouts.md)).

```tsx
// pages/settings/settings.tsx
export const layout = "main";

export default function Settings() {
  const { send, state } = usePaired();
  return <button onClick={() => send("save")}>Save</button>;
}
```

## Layouts and navigation

Shared chrome that wraps pages - navbars, sidebars, status bars - lives in the layouts/ directory and follows the same folder convention. A page picks its layout with `export const layout = "..."`; the full story is on the [Layouts](layouts.md) page. Moving between pages - `navigate`, `Link`, `useRoute`, `isActive` - is the [Routing](routing.md) topic.

## Two styles per page

- **Paired handlers (plain style)**: the UI is React through and through, Go is the backend; state lives in the browser.
- **Tea Model**: the UI logic and state live in Go, React renders it - one language for the whole page, state that survives frontend reloads, and trivially testable Go. See [The Tea model](tea.md).

Mix them per page, or even both on one page.
