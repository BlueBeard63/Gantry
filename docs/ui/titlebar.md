# The TitleBar

The TitleBar is the React half of the window chrome: the drag surface, the title (left, center or right), your own controls, and the window buttons. createApp renders one on every page automatically; this page covers every knob, then replacing it entirely.

## Defaults and Caps

Out of the box you configure nothing: the TitleBar asks the native window which buttons it supports (the bridge's Caps() call) and shows exactly those. Disable minimize in Go and the button disappears; enable maximize and it shows up. In a plain browser tab all window buttons hide and only your content renders.

## Configuring it app-wide: app.tsx

The entry file is synthesized, so TitleBar settings live in an optional app.tsx at the app root, default-exporting CreateAppOptions:

```tsx
// app.tsx
import { goBack, goForward, type CreateAppOptions } from "gantry-web";
import { ArrowLeft, ArrowRight } from "lucide-react";

export default {
  title: "Myapp",
  titleBar: {
    titleAlign: "left", // title next to the buttons instead of centered
    leftReserve: 90,    // widen the clickable left zone (see below)
    left: (
      <>
        <button className="gantry-winbtn" onClick={goBack}><ArrowLeft size={15} /></button>
        <button className="gantry-winbtn" onClick={goForward}><ArrowRight size={15} /></button>
      </>
    ),
  },
} satisfies CreateAppOptions;
```

Whatever app.tsx exports overrides the synthesized defaults; it can also set components, prefix, socketURL and chrome.

## Every option

```tsx
titleBar: {
  titleAlign: "center",              // "left" | "center" | "right"
  left: <NavButtons />,               // left slot: back/forward, menus, an icon
  right: <SyncSpinner />,             // right slot, before the window buttons
  height: 40,                         // MUST match Go CaptionHeight
  leftReserve: 8,                     // MUST match Go CaptionLeftReserve
  rightReserve: 150,                  // MUST match Go CaptionRightReserve
  showMinimize: true,                 // override Caps per button
  showMaximize: false,
  showClose: true,
  closeHint: "Keeps running in the tray", // close button tooltip
  onClose: () => confirmThenClose(),  // replace the close BUTTON's action
  prefix: "gantry",                   // only if Go changed BindingPrefix
  className: "my-titlebar",
}
```

## Slots, custom buttons and styling

- left renders flush against the window's left edge; right renders in its own container just BEFORE the window buttons, visually separate from them (.gantry-titlebar-left / .gantry-titlebar-right are the hooks for your css).
- Style your own top bar buttons with the gantry-tbbtn class - it is independent of the window buttons (gantry-winbtn) and has its own variables: --gantry-tbbtn-w (min width), --gantry-tbbtn-bg, --gantry-tbbtn-fg, --gantry-tbbtn-hover. Or skip the class entirely and style from scratch; the slots do not impose anything.

```tsx
left: <button className="gantry-tbbtn" onClick={goBack}><ArrowLeft size={15} /></button>,
right: <button className="gantry-tbbtn sync">Sync</button>,
```

```css
/* your index.css */
:root { --gantry-tbbtn-hover: rgba(110, 168, 254, 0.15); }
.gantry-titlebar-right .sync { color: var(--gantry-accent); }
```

## Bar height (thin bars)

Every element in the bar follows its height, so a thin bar is just a smaller number on both halves of the contract:

```tsx
// app.tsx
titleBar: { height: 28 },
```

```go
// main.go
Window: func(w *appshell.WindowOptions) { w.CaptionHeight = 28 },
```

Notes:

- The title is pointer-transparent in every alignment, so the bar stays draggable through it; interactive things go in left or right.
- The reserves are the frontend half of the native hit-test contract. The Go window treats CaptionLeftReserve/CaptionRightReserve pixels as clickable instead of draggable - so when you put buttons in the left slot, raise BOTH the leftReserve prop and the window's CaptionLeftReserve (gantry.Config.Window: w.CaptionLeftReserve = 90) or your buttons will drag the window instead of clicking. Same pairing for height/CaptionHeight and rightReserve/CaptionRightReserve. See [The main window](../shell/window.md).
- onClose replaces what the close BUTTON does; the native close path (Alt+F4) still goes through Go's OnCloseRequest, which is where real close policy belongs.

## Per-page chrome

Any page can opt out of the TitleBar (widgets, popups, splash pages):

```tsx
export const chrome = false;
export default function WidgetTimer() { /* ... */ }
```

Or turn it off app-wide and hand-place it: createApp({ chrome: false }), then render <TitleBar /> yourself wherever it belongs.

## Styling

Plain css, themed by the --gantry-* variables (see [Styling](styling.md)): --gantry-titlebar-bg, --gantry-btn-hover, --gantry-close-hover, and the shared fg/font variables. The classes are stable if you want surgical overrides: `.gantry-titlebar, .gantry-titlebar-title, .gantry-winbtn, .gantry-winbtn-close`.

## Fully custom chrome

Skip TitleBar entirely and build your own from the pieces:

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
      {caps?.close && <button onClick={shell.close}>x</button>}
    </div>
  );
}
```

DragStrip is the invisible drag surface (left-button mousedown starts the native move; double-clicks are ignored on purpose). Remember the contract: your chrome's height and button zone must match the Go window's caption metrics.