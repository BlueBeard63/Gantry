# Close behavior and app lifecycle

What happens when the user clicks the close button is a decision, not a default. This page covers the close hook and the App lifecycle helper first, then the runtime toggle, the single-instance guard and shutdown as advanced material.

## OnCloseRequest: intercepting close

Every close request (the X button, Alt+F4, a WM_CLOSE from anywhere) runs your hook first, if you set one:

```go
appshell.WindowOptions{
    // ...
    OnCloseRequest: func() appshell.CloseAction {
        // decide what close means for this app
        return appshell.CloseHide
    },
}
```

Return one of:

- `CloseAllow` - proceed: the window closes and RunWindow returns. This is the behavior when the hook is nil.
- `CloseCancel` - swallow the request; the window stays. Use for "you have unsaved work" flows (tell the frontend to show its dialog, then call CloseMainWindow if the user confirms).
- `CloseHide` - hide the window and keep the app running. The window still exists with all its state; appshell.ShowMainWindow() brings it back instantly.

The hook is the natural place to hand off to something else: hide the window and fire a notification popup ("still running in the tray"), show a widget, whatever fits. One rule: it runs on the window's own thread, so do not block in it - kick real work onto a goroutine.

Two escape hatches bypass the hook by design:

- `appshell.CloseMainWindow()` force-closes (tray Quit uses this, so Quit always works no matter what the hook says).
- `DisableClose` ignores close requests entirely without consulting a hook.

Window geometry saves on any path that actually closes or hides, so `CloseHide` apps keep their position too.

## appshell.App: the standard lifecycle

Most apps do not call `RunWindow` directly; they fill in `appshell.App` and call Run:

```go
shell := &appshell.App{
    Window: appshell.WindowOptions{ /* ... */ },
    Tray: &tray.Options{
        Icon:    appicon.ICO(icon),
        Tooltip: "Myapp is running",
    },
}
return shell.Run(ctx, cancel)
```

Run blocks the main goroutine (the native window requires it) until ctx is done. It handles:

- opening the window on launch and re-opening it from the tray
- the tray icon, with "Open <Title>" prepended and "Quit" appended around whatever Menu items you provide; Quit force-closes the window and cancels ctx
- the minimize-to-tray dance: with a tray, closing the window leaves the app running; without one, closing the window cancels ctx and the app exits
- falling back to the default browser when the native webview is unavailable, and Browser mode (--browser) that skips the window entirely
- `TrayOnly` mode: no main window at all - the app lives in the tray and shows only widgets and popups

Reopening from the tray first tries `ShowMainWindow` (instant if the window was hidden or minimized), and only opens a fresh window if none is alive - so it composes with CloseHide.

## Two ways to "close to tray"

- Window-per-open (the Timekeep model, and App.Run's default): close really closes; the tray opens a new window later. Page state resets, memory is freed while hidden.
- One hidden window: set OnCloseRequest to CloseHide. Reopening is instant and page state survives. Costs the webview staying alive.

Pick per app; both are one line.

---

## Advanced: switching close behavior at runtime

Both models can be flipped while the app runs - the natural home for a "keep running in the tray" settings checkbox.

**Window-per-open apps** (a `gantry.Run` app with `Tray: true`): call `gantry.SetCloseToTray`. It is read at every window close, so the next close follows whatever was set last - `true` (the default) keeps the app in the tray, `false` makes close quit the whole app, tray icon included. The full recipe, from a settings page checkbox down to Go:

```tsx
// pages/settings/settings.tsx
const { send } = usePaired();
<input type="checkbox" checked={tray}
       onChange={(e) => { setTray(e.target.checked); send("tray", e.target.checked); }} />
```

```go
// pages/settings/settings.go
var Page = ui.Page{
    Key: "pages/settings",
    On: ui.Handlers{
        "tray": func(p json.RawMessage) {
            var on bool
            _ = json.Unmarshal(p, &on)
            gantry.SetCloseToTray(on)
            // persist the preference and call SetCloseToTray again at
            // startup (e.g. in Config.Setup) so it survives restarts.
        },
    },
}
```

`gantry.CloseToTray()` reads the current setting. Note the limits: the toggle needs the tray to exist (`Config.Tray` or `--tray`) - a tray cannot be created at runtime - and without one, closing already exits.

**Hidden-window apps** (OnCloseRequest + CloseHide): the hook is invoked fresh on every close, so close over a mutable value instead of returning a constant:

```go
var hideOnClose atomic.Bool // flip from anywhere, e.g. a settings handler

Window: func(w *appshell.WindowOptions) {
    w.OnCloseRequest = func() appshell.CloseAction {
        if hideOnClose.Load() {
            return appshell.CloseHide
        }
        return appshell.CloseAllow
    }
},
```

Apps driving `appshell.App` directly get the same switch as the `KeepRunning func() bool` field - consulted at each close on tray apps; return false and that close quits.

## Advanced: single instance

Gantry apps bind a fixed local port for their frontend server, which doubles as the single-instance guard:

```go
ln, err := appshell.Listen(cfg.Port)
if err != nil {
    // "already running?" - another instance holds the port
    return err
}
```

Starting a second copy fails the bind and exits. There is no lockfile to go stale.

## Advanced: shutdown

The scaffold wires ctx to Ctrl+C (`signal.NotifyContext`) and tray Quit to `cancel()`. When ctx is done: `App.Run` returns, your server shuts down, widgets and popups die with their `ProcManager (CloseAll)`, and main returns. Nothing needs to be killed by hand.
