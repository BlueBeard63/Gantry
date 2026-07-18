# Pages

A page is a routable [pair](pairs.md): a `.tsx`/`.go` folder the router can navigate to by URL. Everything about how the two halves talk - keys, `usePaired`, pushes, registration - is the shared [pairing model](pairs.md); this page is only what makes a pair a *page*: the four fields of `ui.Page` on the Go side, and the three tuning exports on the tsx side.

## The Go half: ui.Page

```go
var Page = ui.Page{
    Key:   "pages/settings",
    Route: "/settings",                        // optional; derived from Key when empty
    Model: func() ui.Model { return model{} }, // optional Tea model
    On: ui.Handlers{                           // optional plain handlers
        "save": func(p json.RawMessage) { /* ... */ },
    },
    Call: ui.Calls{                            // optional awaited calls
        "load": func(p json.RawMessage) (any, error) { return data, nil },
    },
}
```

Those four fields are the entire type:

- **`Key`** - the folder path, the one string that ties the halves together (see [Pairs](pairs.md)). Required.
- **`Route`** - the URL this page answers to. Leave it empty and Gantry derives it from the folder: `pages/settings` -> `/settings`, `pages/index` -> `/` (see [Routing](routing.md)).
- **`Model`** - a factory returning a Tea model to run the whole page as a Go state machine. It is called **once, lazily, the first time the page becomes active**, and its `View()` tree is what the tsx renders through `<TeaView />` (see [The Tea model](tea-model.md)).
- **`On`** - fire-and-forget [paired handlers](pairs.md), `map[string]func(json.RawMessage)`.
- **`Call`** - awaited calls that return `(any, error)` (see [Awaited Go calls](calls.md)).

A page needs a `Model`, handlers, calls, or none of them: a purely static page needs no `.go` logic at all - though the `.go` file must still exist and export `var Page` if it is to be registered. `Model` and `On`/`Call` are not exclusive; a page can run a Tea `Model` and still answer paired handlers and calls. A folder can both BE a page and CONTAIN pages - nesting works to any depth. See [Routing](routing.md) for how folders become URLs.

## The tsx half: default export + three tuning exports

The tsx half's **default export** is the page component. Three optional **named exports** tune how the shell wraps it:

```tsx
// pages/settings/settings.tsx
export const layout = "main";        // shared chrome around this page
export const route = "/preferences"; // override the derived/Go route
export const chrome = false;         // hide the TitleBar on this page

export default function Settings() {
  const { send, state } = usePaired();
  return <button onClick={() => send("save")}>Save</button>;
}
```

- **`chrome`** (`boolean`) - `export const chrome = false` hides the [TitleBar](titlebar.md) on this page, for widgets and popups that are their own surface. A chromeless page also skips the default `main` layout unless it opts back in.
- **`route`** (`string`) - overrides the derived route from the tsx side. If the Go half also sets `Route`, keep the two matching (see [Routing](routing.md)).
- **`layout`** (`boolean | string | string[]`) - picks the shared chrome that wraps the page:
  - a name - `export const layout = "compact"` wraps in `layouts/compact`;
  - an array - `["main", "compact"]` nests them outermost-first: `<Main><Compact><Page/></Compact></Main>`;
  - `false` - no layout, even if a `main` layout exists;
  - `true` - force the `main` layout even on a chromeless page;
  - omitted - the default is `main` if a `layouts/main` exists (chromeless pages default to none).

The full layout story is on the [Layouts](layouts.md) page.

## Layouts and navigation

Shared chrome that wraps pages - navbars, sidebars, status bars - lives in the `layouts/` directory and follows the same folder convention as any pair. A page picks its layout with the `layout` export above; the full story is on the [Layouts](layouts.md) page. Moving between pages - `navigate`, `Link`, `useRoute`, `isActive` - is the [Routing](routing.md) topic.

## Two styles per page

- **Paired handlers (plain style)**: the UI is React through and through, Go is the backend; state lives in the browser. This is `On`/`Call` plus `usePaired`.
- **Tea Model**: the UI logic and state live in Go, React renders it - one language for the whole page, state that survives frontend reloads, and trivially testable Go. See [The Tea model](tea-model.md).

Mix them per page, or even both on one page.
