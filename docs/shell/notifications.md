# Notifications

> On Android, notifications are real system notifications with their own API - see [Mobile > Notifications](../mobile/notifications.md). This page is the desktop popup system.

Gantry desktop notifications are popup windows your app draws itself: frameless, always on top, placed at the top or bottom of a monitor, and unable to steal focus - buttons are clickable but whatever the user was typing in keeps the keyboard. Because they are real app windows rather than shell toasts, Windows Do Not Disturb / Focus Assist cannot suppress them. Use them for things that genuinely must be seen (check-ins, alarms). Two Go types drive them: `appshell.PopupOptions` (the window) and `notify.Notifier` (the dispatcher).

## The pieces

Like widgets, a popup renders one of your pages and runs out of process. The scaffold already generates the `--shellrole` popup branch in `main.go`:

```go
case "popup":
    defer appshell.RoleLog("myapp", "popup")()
    if err := appshell.RunPopup(appshell.PopupOptions{
        AppName:  "myapp",
        URL:      *roleURL,
        Width:    460,
        Height:   140,
        Monitor:  *monitor,
        Position: *position,
    }); err != nil {
        log.Fatalf("popup: %v", err)
    }
    return
```

Make a page for the popup content (`pages/alert/` with `export const chrome = false;`), then show it from anywhere in your app with the `notify` package:

```go
notifier := &notify.Notifier{
    Proc:      procs, // share the app's ProcManager
    BaseURL:   "http://127.0.0.1:" + strconv.Itoa(port),
    Placement: func() (int, string) { return cfg.Monitor, cfg.Position },
}

notifier.Show("/alert")   // replaces any popup currently showing
notifier.Close()          // dismiss
```

## notify.Notifier fields

- `Proc *appshell.ProcManager` - manages the popup child process. Share one `ProcManager` with the app's widgets. `Show` uses its `Replace("popup", ...)` semantics: a new notification supersedes the old.
- `BaseURL string` - prefixes `Show`'s path, e.g. `"http://127.0.0.1:8330"`. The popup loads its page from the MAIN app's server (so its `usePaired` events travel over the websocket to the main process).
- `Placement func() (monitor int, position string)` - returns the monitor index and `"top"`/`"bottom"`. It is called on **every** `Show`, so a settings change applies to the very next popup. `nil` means primary monitor, bottom.
- `Args func(url string, monitor int, position string) []string` - builds the child's argv. `nil` uses the standard scaffold flags: `--shellrole popup --url <BaseURL+path> --monitor N --position top|bottom`. Override it only if your dispatch differs from the scaffold's.

`Show(path)` replaces any visible popup with one rendering `BaseURL + path`; `Close()` calls `Proc.Kill("popup")` to dismiss it.

## PopupOptions fields

- `AppName string` (required) - namespaces the WebView2 data folder and logs.
- `URL string` - the page to load.
- `Width, Height int` (required) - `normalize()` errors if either is `<= 0`.
- `Monitor int` (default 0) - the display index; `-1` is the primary. See [Monitors and icons](monitors-and-icons.md).
- `Position string` (default `"bottom"`) - `"top"` or `"bottom"` of the monitor work area. The popup is always horizontally centered.
- `Margin int` (default 24) - the gap in pixels from the top or bottom work-area edge.
- `AdjustPos func(x, y int) (int, int)` (default nil) - a final nudge given the computed x, y; see [Advanced: sliding around a widget](#advanced-sliding-around-a-widget).
- `BindingPrefix string` (default `"gantry"`) - names the bound JS functions, as on the [main window](window-options.md#bindingprefix-string-default-gantry).
- `ExtraBindings map[string]any` (default nil) - extra JS functions.
- `Icon appshell.IconSource` (default zero value) - the window icon.
- `DataDirRole string` (default `"popup"`) - the WebView2 data-folder suffix; a popup runs separately from the main window, so it MUST use its own folder (the default is already correct).

## Inside the popup page

The page talks to your app the normal way (`usePaired`) - the popup process serves no logic itself. `RunPopup` binds only three functions, so `useShell()` on a popup gives you `close`, `openExternal`, and `setVisible` (no `drag`, no `resize`). To dismiss cleanly:

- `shell.setVisible(false)` hides the window instantly (the `Visible` binding) - do this first, so the teardown that follows can never paint a goodbye flash, since a hidden window has nothing to composite; then
- `shell.close()`, or the app's `notifier.Close()`, ends the process.

## The lighter option: attention

Sometimes a sound and an orange taskbar flash is enough - no popup at all:

```go
notify.Attention()          // from Go
shell.attention()           // from the frontend (main window)
```

`notify.Attention()` calls `appshell.AttentionMainWindow()`: it plays the system notification sound (`PlaySound("SystemNotification")`) and flashes the main window's taskbar button (`FlashWindowEx` with `FLASHW_ALL | FLASHW_TIMERNOFG`) until the user brings the window to the foreground.

---

## Advanced: sliding around a widget

`PopupOptions.AdjustPos` receives the computed x, y and returns the final ones. The classic use: when a widget parks in the same corner, slide the popup below it. `appshell.FindWindowVisible(title)` tells you if a window with that title exists and is currently visible.

```go
AdjustPos: func(x, y int) (int, int) {
    if appshell.FindWindowVisible("Myapp Timer") {
        return x, y + 52
    }
    return x, y
},
```

## Advanced: fallback behavior

If the WebView2 runtime is missing, `RunPopup` opens the same URL in the default browser (`OpenInBrowser`) instead of erroring, so the notification still reaches the user.
