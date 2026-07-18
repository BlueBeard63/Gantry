# The TitleBar

The TitleBar is the React half of the window chrome: the drag surface, the title (left, center or right), your own controls, and the window buttons. `createApp` renders one on every page automatically (unless the page or app opts out). This page covers every knob it takes, the native contract it shares with the Go window, and replacing it entirely.

## Defaults and Caps

Out of the box you configure nothing. The TitleBar asks the native window which buttons it supports - the bridge's `caps()` call, backed by the Go window's `Caps()` binding - and shows exactly those. The mapping is `props.showMinimize ?? caps?.minimize ?? false` (and likewise maximize/close), so an unset prop defers to the window and a plain browser tab (no bridge, all caps false) shows no window buttons at all. The buttons a window advertises come from Go: `DisableMinimize` drops the minimize button, `EnableMaximize` adds maximize/restore, `DisableClose` drops close (see [The main window](../shell/window.md)). `ShellCaps` also reports `platform` (`"windows"`/`"linux"`) and `frameless` if you need to branch on them in custom chrome.

## Configuring it app-wide: app.tsx

The entry file is synthesized, so TitleBar settings live in an optional `app.tsx` at the app root that default-exports `CreateAppOptions`. Whatever it exports is merged *over* the synthesized defaults, so you customize everything without owning the entry file:

```tsx
// app.tsx
import { goBack, goForward, type CreateAppOptions } from "gantry-web";
import { ArrowLeft, ArrowRight } from "lucide-react";

export default {
  title: "Myapp",
  titleBar: {
    titleAlign: "left", // title sits next to the left slot instead of centered
    leftReserve: 90,    // widen the clickable left zone (see the contract below)
    left: (
      <>
        <button className="gantry-tbbtn" onClick={goBack}><ArrowLeft size={15} /></button>
        <button className="gantry-tbbtn" onClick={goForward}><ArrowRight size={15} /></button>
      </>
    ),
  },
} satisfies CreateAppOptions;
```

Besides `title` and `titleBar`, `CreateAppOptions` also carries `components` (extra Tea components), `prefix` (the binding prefix, if Go changed it), `socketURL`, `chrome` (hide the bar app-wide), and `errors`.

## Every TitleBar option

`titleBar` is a `Partial<TitleBarProps>`; every field with its default:

```tsx
titleBar: {
  titleAlign: "center",              // "left" | "center" | "right" (default "center")
  left: <NavButtons />,               // left slot: back/forward, menus, an icon
  right: <SyncSpinner />,             // right slot, rendered BEFORE the window buttons
  height: 40,                         // px; MUST match Go CaptionHeight (default 40)
  leftReserve: 8,                     // px clickable on the left; MUST match Go CaptionLeftReserve
  rightReserve: 150,                  // px clickable on the right; MUST match Go CaptionRightReserve
  showMinimize: true,                 // override Caps() per button (default: defer to Caps)
  showMaximize: false,
  showClose: true,
  closeHint: "Keeps running in the tray", // native tooltip on the close button
  onClose: () => confirmThenClose(),  // replace what the close BUTTON does
  prefix: "gantry",                   // only if the Go side changed BindingPrefix
  className: "my-titlebar",           // extra class on the .gantry-titlebar root
}
```

## Slots, custom buttons and styling

The bar is a flex row of three regions: `left` renders flush against the window's left edge (`.gantry-titlebar-left`, which also holds a left-aligned title); the window buttons cluster at the far right inside `.gantry-titlebar-end`; and `right` renders in `.gantry-titlebar-right`, in its own container *just before* the window buttons, visually separate from them. A centered title is absolutely positioned across the whole bar (`.gantry-titlebar-title`) and is **pointer-transparent in every alignment**, so the bar stays draggable through the title - interactive things go in `left` or `right`, never in the title.

Style your own bar buttons with the `gantry-tbbtn` class - independent of the window buttons (`gantry-winbtn`) and driven by its own variables so you can theme it without touching the window controls: `--gantry-tbbtn-w` (min width, default 40px), `--gantry-tbbtn-bg` (default transparent), `--gantry-tbbtn-fg` (default the titlebar fg), and `--gantry-tbbtn-hover` (falls back to `--gantry-btn-hover`). Or skip the class and style from scratch; the slots impose nothing.

```tsx
left: <button className="gantry-tbbtn" onClick={goBack}><ArrowLeft size={15} /></button>,
right: <button className="gantry-tbbtn sync">Sync</button>,
```

```css
/* your index.css */
:root { --gantry-tbbtn-hover: rgba(110, 168, 254, 0.15); }
.gantry-titlebar-right .sync { color: var(--gantry-accent); }
```

Everything is themed by the `--gantry-*` variables like the rest of the app (see [Styling](styling.md)): `--gantry-titlebar-bg`, `--gantry-titlebar-fg`, `--gantry-btn-hover`, `--gantry-close-hover`, plus the shared fg/font variables. The stable classes for surgical overrides: `.gantry-titlebar`, `.gantry-titlebar-title`, `.gantry-titlebar-text` (left/right-aligned title), `.gantry-winbtn`, and `.gantry-winbtn-close`. The window buttons use lucide icons (`Minus`, `Square`/`Copy` for maximize/restore, `X`).

## Bar height (thin bars)

Every element in the bar follows its height, so a thin bar is one smaller number on both halves of the contract - the frontend prop and the Go caption metric:

```tsx
// app.tsx
titleBar: { height: 28 },
```

```go
// main.go
Window: func(w *appshell.WindowOptions) { w.CaptionHeight = 28 },
```

## Per-page chrome

Any page can opt out of the TitleBar (widgets, popups, splash screens) by exporting `chrome = false`; that page also skips the default layout, since a chromeless page is its own surface (see [Layouts](layouts.md)):

```tsx
export const chrome = false;
export default function WidgetTimer() { /* ... */ }
```

Or turn the bar off app-wide with `chrome: false` in `app.tsx`/`createApp` and hand-place `<TitleBar />` yourself wherever it belongs.

## The native hit-test contract

The reserves are the frontend half of a two-sided native hit-test. The Go window treats `CaptionLeftReserve`/`CaptionRightReserve` pixels (from the left and right edges of a `CaptionHeight`-tall band) as **clickable** rather than draggable; everything else in that band is drag surface. So when you put buttons in the left slot, raise **both** the `leftReserve` prop and the window's `CaptionLeftReserve` - or your buttons sit in the drag zone and drag the window instead of clicking:

```go
Window: func(w *appshell.WindowOptions) {
    w.CaptionHeight = 40         // pair with titleBar.height
    w.CaptionLeftReserve = 90    // pair with titleBar.leftReserve
    w.CaptionRightReserve = 150  // pair with titleBar.rightReserve
},
```

The Go metrics default to `40` / `8` / `150` (matching the TitleBar prop defaults), with `ResizeMargin` at `6` for the edge-resize zone. `onClose` replaces only what the close *button* does; the native close path (Alt+F4, a `WM_CLOSE` from anywhere) still runs through Go's `OnCloseRequest`, which returns `CloseAllow`/`CloseCancel`/`CloseHide` - that is where real close policy (confirm, minimize-to-tray) belongs.

## Advanced: fully custom chrome

Skip `TitleBar` entirely and assemble your own from the pieces gantry-web exports - `DragStrip`, `useShell`, and `useShellCaps`:

```tsx
import { DragStrip, useShell, useShellCaps } from "gantry-web";

export default function MyChrome() {
  const shell = useShell();
  const caps = useShellCaps();
  return (
    <div className="my-chrome">
      <DragStrip height={48} rightInset={100} />
      <span className="brand">MYAPP</span>
      {caps?.minimize && <button onClick={shell.minimize}>_</button>}
      {caps?.maximize && <button onClick={shell.maximize}>[]</button>}
      {caps?.close && <button onClick={shell.close}>x</button>}
    </div>
  );
}
```

`DragStrip` is the invisible drag surface: a left-button `mousedown` starts the native move (`shell.drag()`), and it deliberately ignores non-left buttons and double-clicks (`e.detail > 1`) so a quick double-tap can't trip the OS caption-maximize. Its props (`height`, `leftInset`, `rightInset`) are the same caption metrics, and it renders `null` outside a native window. `useShell()` is the full bridge (`minimize`, `maximize`, `restore`, `close`, `isMaximized`, `caps`, `attention`, `openExternal`, ...); `useShellCaps()` returns `null` while resolving, then the caps - so the same height/reserve contract applies to your custom chrome exactly as it does to `TitleBar`.
