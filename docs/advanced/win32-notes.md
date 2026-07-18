# Win32 notes

The hard-won lessons baked into appshell's Windows backend (`appshell/*_windows.go`). You do not need any of this to use Gantry - it is documented so future work on the framework does not relearn it the painful way. Every rule here was a real bug once. The handles and constants all live in `winapi_windows.go`; the window logic in `window_windows.go` and `frameless_windows.go`.

## GWL sign extension

`GetWindowLongPtrW` / `SetWindowLongPtrW` take negative index constants (`GWL_STYLE` is -16, `GWLP_WNDPROC` is -4, `GWL_EXSTYLE` is -20). On 64-bit, passing the 32-bit two's-complement form (`0xFFFFFFF0`) makes the calls fail SILENTLY - no error, no effect. The symptoms were surreal: the caption never stripped (a white bar) and the subclass never installed (no dragging). The constants must be full-width, sign-extended uintptrs, written with bitwise-NOT so they are correct at any word size:

```go
gwlStyle    = ^uintptr(15) // -16
gwlpWndProc = ^uintptr(3)  // -4
gwlExStyle  = ^uintptr(19) // -20
```

The same trick names the topmost Z-order handles: `hwndTopmost = ^uintptr(0)` (HWND -1), `hwndNotopmost = ^uintptr(1)` (HWND -2).

## Never Terminate (or SendMessage) inside a JS binding

A bound Go function (`w.Bind(...)`) runs in the middle of a JS dispatch on the window's message loop. Tearing the webview down from there (`Terminate`, or `DestroyWindow` via `SendMessage`) crashes the process; entering a modal loop (a `SendMessage` of `WM_NCLBUTTONDOWN` for dragging) hangs it. The rule: bindings only ever **PostMessage**. Close is an async `WM_CLOSE` (`closeWindow` -> `PostMessageW`); drag is `ReleaseCapture` plus a posted `WM_NCLBUTTONDOWN` (`dragWindow`). The message loop picks them up after the dispatch unwinds.

The same hazard bites show-state changes: the Minimize/Maximize/Restore bindings use `ShowWindowAsync`, never `ShowWindow`. A synchronous show-state change re-enters the WndProc (a `WM_SIZE`/`WM_NCCALCSIZE` cascade) mid-dispatch - the same crash family, seen as intermittent crashes when window buttons are hit during rapid UI activity.

## The drag and resize hand-off

The WebView2 child window covers the whole client area, so the parent's `WM_NCHITTEST` caption band and resize margin never see the mouse - the web page gets the mousedown instead. Two frontend pieces hand control back to the native window:

- **Drag:** `gantry-web`'s TitleBar calls the `Drag` binding on mousedown; `dragWindow` releases Chromium's mouse capture and posts a caption click (`WM_NCLBUTTONDOWN` with `HTCAPTION`). The native move loop takes over - snapping included, exactly like a title-bar drag.
- **Resize:** `gantry-web`'s `ResizeFrame` renders eight invisible fixed-position edge strips (5px edges, 10px corners) that carry the resize cursors, and calls the `ResizeEdge` binding on mousedown. `resizeWindow` maps the edge name (`"n"`, `"se"`, ...) to its hit-test code and posts `WM_NCLBUTTONDOWN` with it, starting the native interactive resize.

Note both platforms use `ResizeFrame` now (Linux hands off to the compositor instead). The subclass still returns the 8-direction resize zones from `WM_NCHITTEST` as a backstop, but the child window swallows the hover in practice, which is why the frontend strips exist at all.

## Frameless windows and WM_NCCALCSIZE

Returning 0 from `WM_NCCALCSIZE` (with `wparam != 0`) makes the client area the whole window - that is the entire frameless trick, and it is what the `subclassWindow` proc does first. Everything the frame used to do must then be re-implemented in the same subclass:

- `WM_NCHITTEST` returns the resize zones (`resizeMargin`, default 6px, in 8 directions - `HTTOPLEFT`..`HTBOTTOMRIGHT`) when not maximized, then `HTCAPTION` for the draggable top band (bounded by `captionHeight`, and the left/right reserves that keep the frontend's buttons clickable), else `HTCLIENT`.
- `WM_GETMINMAXINFO` clamps a maximized window to the monitor WORK AREA (via `MonitorFromWindow` + `GetMonitorInfoW`) - without that clamp maximize covers the taskbar, because there is no frame overhang for Windows to hide off-screen - and enforces the min/max track sizes.
- `WM_EXITSIZEMOVE` and a `WM_SIZE` with `SIZE_MAXIMIZED` snapshot the geometry mid-session, so a size survives a kill without a clean close (e.g. `gantry dev` tearing the app down on hot reload). Plain drag-resizes are left to `WM_EXITSIZEMOVE` to avoid a geometry write per pixel.
- `WM_CLOSE` runs the close hook: `DisableClose` swallows it, `OnCloseRequest` can return cancel/hide, and a proceeding close (or a `forceClose` set by `CloseMainWindow`) fires the geometry snapshot before falling through to the original proc.

## Widget style rewrites: three rules

1. **Never clear `WS_VISIBLE` with a raw style write.** It hides the window logically without telling the DWM, which keeps compositing the last (unpainted, white) frame as an untouchable ghost on screen. Hide with a real `ShowWindow(SW_HIDE)` - `setVisible` does exactly this (`SW_SHOWNA` to show without activating, `SW_HIDE` to hide).
2. **Always OR into `GWL_EXSTYLE`, never replace it.** WebView2 sets `WS_EX_NOREDIRECTIONBITMAP` on the window; strip it and the composited content goes white (invisible page, busy cursor).
3. **Hidden windows destroy cleanly.** A popup that hides itself before its process is killed can never paint a goodbye flash frame.

## WebView2 user-data folders

Every process needs its own browser-data folder. Two processes sharing one means the second environment fails to initialize - historically "the popup never appeared at all". `webviewDataPath` keys folders by AppName plus role (`%LocalAppData%\<app>\webview-<role>`), which is also why AppName is a required option. This is the concrete reason widgets and popups run as separate processes (see [Architecture](architecture.md)); the role is `main` for the main window and `widget-<title>` / `popup` for helpers.

## Spawning GUI children

Console-attached parents (dev builds, built without `-H windowsgui`) flash a console for each child unless `CREATE_NO_WINDOW` is set - `hideSpawnConsole` sets exactly that in `SysProcAttr.CreationFlags`. Do NOT also set `STARTUPINFO` `wShowWindow = SW_HIDE` (`HideWindow`): that overrides the child GUI window's first `ShowWindow`, and the WebView2 content comes up as an unpainted white rectangle. Because `-H windowsgui` children have no console, a crashing role is otherwise undiagnosable - `RoleLog` redirects the child's standard logger to `%LocalAppData%\<app>\<role>.log`.

## syscall.NewCallback is forever

Callbacks created with `syscall.NewCallback` are never freed, and the process has a hard cap on how many can exist. Create one callback per window role and reuse it - `subclassWindow` allocates exactly one permanent callback for the frameless chrome, resize, size limits and close hook combined, chaining to the original WndProc with `CallWindowProcW`. Gantry's per-process window model keeps the count trivially low, but if you ever host many windows in one process, do not create callbacks in a loop.

## Attention without focus stealing

`FlashWindowEx` with `FLASHW_ALL | FLASHW_TIMERNOFG` flashes the caption and taskbar button until the window is foregrounded, and `PlaySoundW` with the `"SystemNotification"` alias (`SND_ALIAS | SND_ASYNC | SND_NODEFAULT`) plays the system notification sound. Together (`attention` / `AttentionMainWindow`, also reachable from the frontend via the `Attention` binding) they say "something happened" without yanking focus - which Windows would (rightly) block anyway, since `SetForegroundWindow` from a background process is a no-op.
