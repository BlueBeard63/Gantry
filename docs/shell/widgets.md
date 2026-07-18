# Widgets

A widget is a small always-on-top helper window: a floating timer, a peek card, a mini toolbar. It renders one of your app's pages in its own frameless native window, and it runs as its own process so a renderer crash can never take your app down.

## The pieces

1. A page to render - widgets are just routes. Make a pages/widget-timer/ pair like any other page. Export `export const chrome = false;` from its tsx module so it does not get a TitleBar.
2. A --shellrole branch in main.go that runs the widget window instead of the app:

```go
case "widget-timer":
    defer appshell.RoleLog("myapp", "widget-timer")()
    if err := appshell.RunWidget(appshell.WidgetOptions{
        AppName:    "myapp",
        Title:      "Myapp Timer", // must be unique per widget kind
        URL:        "http://127.0.0.1:" + strconv.Itoa(*port) + "/widget-timer",
        Width:      300,
        Height:     44,
        Placement:  appshell.PlaceTopCenter,
        NoActivate: true,
    }); err != nil {
        log.Fatalf("widget: %v", err)
    }
    return
```

3. A ProcManager in the app that spawns it:

```go
widgets := appshell.NewProcManager()
widgets.Show("timer", "--shellrole", "widget-timer", "--port", strconv.Itoa(port))
defer widgets.CloseAll()
```

Show starts it if not running, Toggle flips it (tray left-click patterns), Replace restarts it, Kill stops it, Running asks.

## WidgetOptions

- AppName, Title, URL - identity and content. Title doubles as the singleton guard: opening a widget closes any predecessor with the same title, even an orphan from a killed app. Keep titles unique per widget kind.
- Width, Height - required; widgets have no sensible default size.
- Placement - PlaceTopCenter, PlaceTopLeft, PlaceTopRight, PlaceBottomLeft, PlaceBottomCenter, PlaceBottomRight, PlaceCenter, or PlaceCustom with X and Y. Margin (default 16) is the gap from the work-area edge. Monitor picks the display (-1 = primary); see [Monitors and icons](monitors-and-icons.md).
- NoActivate - the widget never steals keyboard focus. Turn this on for glanceable things (timers): clicks work, typing elsewhere is never interrupted.
- CloseOnDeactivate - the widget dismisses itself when it loses activation, like a shell flyout: click anywhere else and it is gone. (This one wants activation, so do not combine with NoActivate.)
- StartHidden - create hidden; the page reveals itself with setVisible(true) when it has something to show.
- SquareCorners - opt out of the Win11 rounded corners.
- BindingPrefix, ExtraBindings, Icon, DataDirRole - as on the [main window](window.md); DataDirRole defaults to a folder derived from Title.

## The widget bridge

Inside a widget page, useShell() gives you:

- shell.drag() - start a native move on mousedown (put a grip element in the corner and call it there; widgets have no caption band).
- shell.close() - close the widget window.
- shell.setVisible(show) - show/hide the native window from the page. A timer widget shows itself only while something is running.
- shell.resize(w, h) - resize in place, keeping whatever position the user dragged it to. Flyouts grow downward with this.

```tsx
export const chrome = false;

export default function WidgetTimer() {
  const shell = useShell();
  const [open, setOpen] = useState(false);
  return (
    <div className="timer">
      <span className="grip" onMouseDown={(e) => {
        if (e.button === 0) { e.preventDefault(); shell.drag(); }
      }}>::</span>
      <span>00:25:13</span>
      <button onClick={() => {
        shell.resize(300, open ? 44 : 84);
        setOpen(!open);
      }}>{open ? "^" : ">"}</button>
    </div>
  );
}
```

---

## Advanced: why a separate process

The webview renderer is a big piece of software; if it ever crashes, only the widget process dies - your scheduler, server and main window never notice. It also sidesteps a WebView2 rule: each process needs its own browser-data folder, which WidgetOptions derives from the Title automatically. RoleLog writes the child's log to `%LocalAppData%\<AppName>\<role>.log`, because a windowsgui child has no console to print to.
