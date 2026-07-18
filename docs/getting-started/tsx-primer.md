# A TSX primer

Just enough React and TypeScript to build Gantry pages, aimed at someone new to them. This is background, not a full tutorial - skim it, then learn the rest by reading the code `gantry new` generates. The snippets here are drawn from the actual scaffold. If you already know React, skip to [Your first app](first-app.md).

## What TSX is

A `.tsx` file is TypeScript (JavaScript plus types) that can also contain HTML-looking markup called **JSX**. The markup is not a template language - it compiles to real function calls that build UI elements:

```tsx
const el = <h1>Hello</h1>;
```

Vite (the build tool the gantry CLI drives for you) compiles TSX to plain JavaScript that WebView2 runs. You never configure Vite yourself - the CLI synthesizes its config. Your `.tsx` files live inside pair folders next to their `.go` half; see [Project structure](project-structure.md).

## Components and the default export

A component is a function that returns JSX, with an **Uppercase** name. Every page's `.tsx` exports one as its `default` - that is the contract Gantry looks for:

```tsx
export default function Index() {
  return <div className="index-page">Hello from the index page</div>;
}
```

Components nest like tags and pass values called **props**:

```tsx
function Badge(props: { label: string }) {
  return <span className="badge">{props.label}</span>;
}

// used elsewhere, self-closing because it has no children:
<Badge label="new" />
```

Curly braces inside markup mean "evaluate this expression and drop the result in": `{count}`, `{user.name}`, `{items.length > 0 ? "yes" : "no"}`. Two gotchas coming from HTML: the attribute is `className`, not `class`, and every tag must close - `<br />`, `<input ... />`, not `<br>`. The scaffold's `Index` component composes small local components (`GantryLogo`, `ReactLogo`) exactly this way.

## State with useState

State is data that changes while the app runs. When it changes, React re-renders the component. `useState` hands you the current value and a setter:

```tsx
import { useState } from "react";

export default function Index() {
  const [count, setCount] = useState(0);
  return <button onClick={() => setCount(count + 1)}>count is {count}</button>;
}
```

Never reassign `count` directly - always call `setCount`, or React will not know to re-render. `onClick={() => ...}` passes a function React calls on click; note the arrow function, so it runs on click rather than during render.

## Effects with useEffect

`useEffect` runs code **after** render - subscriptions, timers, anything that touches the outside world. Return a function to clean up:

```tsx
import { useEffect } from "react";

useEffect(() => {
  const t = setInterval(tick, 1000);
  return () => clearInterval(t); // cleanup when the component goes away
}, []); // [] = run once when the component first appears
```

In Gantry you mostly will not write raw effects - `usePaired()` already wraps the common one (subscribing to what Go pushes) for you.

## Lists and keys

Render a list with `.map`, and give each item a stable `key` so React can track identity across re-renders:

```tsx
<ul>
  {todos.map((t) => (
    <li key={t.id}>{t.text}</li>
  ))}
</ul>
```

## TypeScript in one paragraph

Types describe the shape of values: `string`, `number`, `boolean`, arrays (`string[]`), and object shapes (`{ name: string }`). You mostly write them on component props and `useState` when the initial value is ambiguous; everything else is inferred. If the editor draws a red squiggle, read the message - it is usually "this value might not have the shape you think". The generated `tsconfig.json` turns on `strict` mode and points at `gantry-web/types`, so imports from `gantry-web` are fully typed and autocompleted.

## The Gantry pieces

Almost everything Gantry-specific comes from the `gantry-web` package (installed by `gantry new`). The three you reach for constantly:

```tsx
import { usePaired, Link, ExternalLink } from "gantry-web";
import { TeaView } from "gantry-web/tea";
```

- **`usePaired()`** is the channel to the `.go` file sitting next to your `.tsx`. Call it with **no argument** inside a pages/ or components/ folder - the Gantry Vite plugin fills in the key from the folder path. It returns `{ send, call, on, state }`:

```tsx
export default function Index() {
  const { send, state } = usePaired();
  return (
    <button onClick={() => send("buttonPress", 1)}>
      {/* send() fires a handler in index.go; state mirrors what Go pushes */}
      go pushed: {JSON.stringify(state)}
    </button>
  );
}
```

  `send(event, payload)` fires a Go handler (`ui.Handlers`), `call(name, payload)` awaits a Go function that returns a value (`ui.Calls`), `on(event, fn)` subscribes to pushes, and `state` is the latest value Go pushed under the event name `"state"`. See [Pairs](../ui/pairs.md).

- **`TeaView`** renders a page whose UI logic lives in Go - it draws whatever that page's `View()` returns and needs no props. In a Tea-style page the `.tsx` is just chrome around `<TeaView />`:

```tsx
import { TeaView } from "gantry-web/tea";

export default function Index() {
  return (
    <div className="card">
      <TeaView />
    </div>
  );
}
```

  See [The Tea model](../ui/tea-model.md).

- **`Link`** and **`navigate`** move between pages without a reload. Use `<Link to="/settings">Settings</Link>` in markup, or `navigate("/settings")` from an event handler. `ExternalLink` opens a URL in the real browser instead of inside the app window. (Routing only kicks in once your app has more than one page - see [Project structure](project-structure.md).)

Other `gantry-web` exports you will meet as you go: `resourceURL()` for files in `resources/`, `useMode()`/`useArg()` for build mode and app arguments, and `TitleBar` for the window's top bar.

Next: [Your first app](first-app.md).
