# The TitleBar

The TitleBar is the React half of the window chrome: the drag surface,
an optional title, and the window buttons. createApp renders one on
every page automatically; this page covers customizing or replacing it.

## Defaults and Caps

Out of the box you configure nothing: the TitleBar asks the native
window which buttons it supports (the bridge's Caps() call) and shows
exactly those. Disable minimize in Go and the button disappears; enable
maximize and it shows up. In a plain browser tab all window buttons
hide and only your content renders.

## Props

```tsx
<TitleBar
  title={<span>Myapp - draft 3</span>} // centered, pointer-transparent
  left={<MenuButton />}                 // left slot, clickable
  right={<SyncSpinner />}               // right slot, before the buttons
  height={40}                           // MUST match Go CaptionHeight
  rightReserve={150}                    // MUST match Go CaptionRightReserve
  showMinimize={true}                   // override Caps per button
  showMaximize={false}
  showClose={true}
  closeHint="Keeps running in the tray" // close button tooltip
  onClose={() => confirmThenClose()}    // replace the default close
  prefix="gantry"                       // only if Go changed BindingPrefix
  className="my-titlebar"
/>
```

Notes:

- title is pointer-transparent so the center of the bar stays
  draggable; put interactive things in left or right instead.
- height and rightReserve are the frontend half of the native hit-test
  contract - change them together with the Go window's CaptionHeight /
  CaptionRightReserve or clicks and drags stop lining up. See
  [The main window](../shell/window.md).
- onClose replaces what the close BUTTON does; the native close path
  (Alt+F4) still goes through Go's OnCloseRequest, which is where real
  close policy belongs.

## Per-page chrome

Any page can opt out of the TitleBar (widgets, popups, splash pages):

```tsx
export const chrome = false;
export default function WidgetTimer() { /* ... */ }
```

Or turn it off app-wide and hand-place it: createApp({ chrome: false }),
then render <TitleBar /> yourself wherever it belongs.

## Styling

Plain css, themed by the --gantry-* variables (see
[Styling](styling.md)): --gantry-titlebar-bg, --gantry-btn-hover,
--gantry-close-hover, and the shared fg/font variables. The classes are
stable if you want surgical overrides: .gantry-titlebar,
.gantry-titlebar-title, .gantry-winbtn, .gantry-winbtn-close.

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

DragStrip is the invisible drag surface (left-button mousedown starts
the native move; double-clicks are ignored on purpose). Remember the
contract: your chrome's height and button zone must match the Go
window's caption metrics.
