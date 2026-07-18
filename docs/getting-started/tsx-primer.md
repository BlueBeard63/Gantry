# A TSX primer

Just enough React and TypeScript to build Gantry pages, aimed at someone new to them. This is background, not a tutorial - skim it, then learn the rest by reading the code `gantry new` generates. If you know React, skip to [Your first app](first-app.md).

## What TSX is

TSX files are TypeScript (JavaScript with types) that can also contain HTML-looking markup called JSX. The markup is not a template language - it is real code that builds UI elements:

```tsx
const el = <h1>Hello</h1>;
```

Vite (the build tool the gantry CLI drives for you) compiles TSX to plain JavaScript the webview runs.

## Components

A component is a function that returns markup. Component names start uppercase. Your page files export one as their default:

```tsx
export default function Index() {
  return <div>Hello from the index page</div>;
}
```

Components can use other components like tags, and pass them values called props:

```tsx
function Badge(props: { label: string }) {
  return <span className="badge">{props.label}</span>;
}

// elsewhere
<Badge label="new" />
```

Curly braces inside markup mean "evaluate this expression": `{count}, {user.name}, {items.length > 0 ? "yes" : "no"}`.

Two gotchas coming from HTML: it is className (not class), and every tag must close (`<br />` not `<br>`).

## State with useState

State is data that changes while the app runs. When state changes, React re-renders the component. useState gives you a value and a function that changes it:

```tsx
import { useState } from "react";

export default function Index() {
  const [count, setCount] = useState(0);
  return <button onClick={() => setCount(count + 1)}>count is {count}</button>;
}
```

Never assign to count directly - always call setCount, or React will not know to re-render.

## Effects with useEffect

`useEffect` runs code after render - subscriptions, timers, anything that talks to the outside world. Return a function to clean up:

```tsx
useEffect(() => {
  const t = setInterval(tick, 1000);
  return () => clearInterval(t);
}, []); // [] = run once when the component appears
```

In Gantry you mostly will not need raw effects - `usePaired` wraps the common one (subscribing to Go pushes) for you.

## Lists and keys

Render lists with map, and give each item a stable key so React can track identity:

```tsx
<ul>
  {todos.map((t) => (
    <li key={t.id}>{t.text}</li>
  ))}
</ul>
```

## TypeScript in one paragraph

Types describe the shape of values: `string`, `number`, `boolean`, arrays (`string[]`), and `object` shapes (`{ name: string }`). You mostly write them on function props; everything else is inferred. If the editor draws a red squiggle, read the message - it is usually "this thing might not have the shape you think".

## The Gantry pieces

Gantry adds three imports you will use constantly:

```tsx
import { usePaired, navigate } from "gantry-web";
import { TeaView } from "gantry-web/tea";
```

- `usePaired()` talks to the `.go` file next to your `.tsx`: `send()` fires its handlers, state mirrors what Go pushes. See [Pairs](../ui/pairs.md).
- `navigate("/settings")` switches pages without a reload.
- `TeaView` renders a page whose UI logic lives in Go. See [The Tea model](../ui/tea.md).

Next: [Your first app](first-app.md).
