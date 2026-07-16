# Notifications

> On Android, notifications are real system notifications with their own API - see [Mobile > Notifications](../mobile/notifications.md). This page is the desktop popup system.

Gantry notifications are popup windows your app draws itself: frameless, always on top, placed at the top or bottom of a monitor, and unable to steal focus - buttons are clickable but whatever the user was typing in keeps the keyboard. Because they are real app windows rather than shell toasts, Windows Do Not Disturb / Focus Assist cannot suppress them. Use them for things that genuinely must be seen (check-ins, alarms).

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

Make a page for the popup content (pages/alert/ with `export const chrome = false;`), then show it from anywhere in your app with the notify package:

```go
notifier := &notify.Notifier{
    Proc:    procs, // share the app's ProcManager
    BaseURL: "http://127.0.0.1:" + strconv.Itoa(port),
    Placement: func() (int, string) { return cfg.Monitor, cfg.Position },
}

notifier.Show("/alert")   // replaces any popup currently showing
notifier.Close()          // dismiss
```

Show spawns "<exe> --shellrole popup --url <BaseURL+path> --monitor N --position top|bottom" with replace semantics: a new notification supersedes the old one. Placement is evaluated at each Show, so a settings change applies to the very next popup.

## PopupOptions

- Width, Height - required.
- Position - "top" or "bottom" (default) of the monitor work area; Margin (default 24) is the gap from the edge.
- Monitor - display index, -1 = primary.
- AdjustPos - a final nudge given the computed x, y. The classic use: when a widget parks in the same corner, slide the popup below it. `appshell.FindWindowVisible(title)` tells you if the widget is currently on screen.

```go
AdjustPos: func(x, y int) (int, int) {
    if appshell.FindWindowVisible("Myapp Timer") {
        return x, y + 52
    }
    return x, y
},
```

## Inside the popup page

The page talks to your app the normal way (`usePaired` - the popup process serves no logic itself, events go to the main app over the websocket... note the popup loads the page from the MAIN app's server, so its events land in the main process). To dismiss:

- `shell.setVisible(false)` hides the window instantly (nice before the process is torn down - a hidden window can never paint a goodbye flash), then
- `shell.close()` or the app's `notifier.Close()` ends it.

## The lighter option: attention

Sometimes a sound and an orange taskbar flash is enough:

```go
notify.Attention()          // from Go
shell.attention()           // from the frontend
```

It plays the system notification sound and flashes the main window's taskbar button until the user brings it to the foreground.

## Fallback behavior

If the WebView2 runtime is missing, RunPopup opens the same URL in the default browser instead, so the notification still reaches the user.
