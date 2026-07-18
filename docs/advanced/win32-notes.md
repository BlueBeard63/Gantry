# Win32 notes

The hard-won lessons baked into appshell's Windows backend. You do not need any of this to use Gantry - it is documented so future work on the framework does not relearn it the painful way. Every rule here was a real bug once.

## GWL sign extension

`GetWindowLongPtrW` / `SetWindowLongPtrW` take negative index constants (`GWL_STYLE` is -16, `GWLP_WNDPROC` is -4, `GWL_EXSTYLE` is -20). On 64-bit, passing the 32-bit two's-complement form (`0xFFFFFFF0`) makes the calls fail SILENTLY - no error, no effect. The symptoms were surreal: the caption never stripped (a white bar) and the subclass never installed (no dragging). The constants must be full-width, sign-extended uintptrs:

```go
gwlStyle    = ^uintptr(15) // -16
gwlpWndProc = ^uintptr(3)  // -4
gwlExStyle  = ^uintptr(19) // -20
```

## Never Terminate (or SendMessage) inside a JS binding

A bound Go function runs in the middle of a JS dispatch on the window's message loop. Tearing the webview down from there (Terminate, or DestroyWindow via SendMessage) crashes the process; entering a modal loop (SendMessage of `WM_NCLBUTTONDOWN` for dragging) hangs it. The rule: bindings only ever **PostMessage**. Close is an async `WM_CLOSE`; drag is `ReleaseCapture` plus a posted `WM_NCLBUTTONDOWN`. The message loop picks them up after the dispatch unwinds.

## The drag hand-off

The WebView2 child window covers the whole client area, so the parent's `WM_NCHITTEST` caption band never sees the mouse - the web page gets the mousedown instead. The custom top bar therefore calls the Drag binding, which releases Chromium's mouse capture and posts a caption click; the native move loop takes over from there, and everything feels exactly like a title-bar drag, snapping included.

## Widget style rewrites: three rules

1. **Never clear `WS_VISIBLE` with a raw style write.** It hides the window logically without telling the DWM, which keeps compositing the last (unpainted, white) frame as an untouchable ghost on screen. Hide with a real `ShowWindow(SW_HIDE)`.
2. **Always OR into `GWL_EXSTYLE`, never replace it.** WebView2 sets `WS_EX_NOREDIRECTIONBITMAP` on the window; strip it and the composited content goes white (invisible page, busy cursor).
3. **Hidden windows destroy cleanly.** A popup that hides itself before its process is killed can never paint a goodbye flash frame.

## Frameless windows and WM_NCCALCSIZE

Returning 0 from `WM_NCCALCSIZE` (with `wparam != 0`) makes the client area the whole window - that is the entire frameless trick. Everything the frame used to do must then be re-implemented: `WM_NCHITTEST` returns the resize zones (6px edges, 8 directions) and `HTCAPTION` for the drag band, and `WM_GETMINMAXINFO` clamps a maximized window to the monitor WORK AREA - without that clamp, maximize covers the taskbar, because there is no frame overhang for Windows to hide off-screen.

## WebView2 user-data folders

Every process needs its own browser-data folder. Two processes sharing one means the second environment fails to initialize - historically "the popup never appeared at all". Gantry keys folders by AppName plus role (`%LocalAppData%\<app>\webview-<role>`), which is also why AppName is a required option. This is the concrete reason widgets and popups run as separate processes (see [Architecture](architecture.md)).

## Spawning GUI children

Console-attached parents (dev builds) flash a console for each child unless `CREATE_NO_WINDOW` is set. Do NOT also set `STARTUPINFO` `wShowWindow = SW_HIDE`: that overrides the child GUI window's first `ShowWindow`, and the WebView2 content comes up as an unpainted white rectangle.

## syscall.NewCallback is forever

Callbacks created with `syscall.NewCallback` are never freed, and the process has a hard cap on how many can exist. Create one callback per window role and reuse it (chaining to the original WndProc with `CallWindowProcW`). Gantry's per-process window model keeps the count trivially low, but if you ever host many windows in one process, do not create callbacks in a loop.

## Attention without focus stealing

`FlashWindowEx` with `FLASHW_ALL | FLASHW_TIMERNOFG` flashes the taskbar button until the window is foregrounded, and `PlaySoundW` with `SND_ALIAS` plays the system notification sound. Together they say "something happened" without yanking focus - which Windows would (rightly) block anyway, since `SetForegroundWindow` from a background process is a no-op.
