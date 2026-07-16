// Desktop apps do not zoom. The webview inherits browser zoom gestures
// (Ctrl+wheel, Ctrl +/-/0), which feel broken in something pretending to
// be a native window - suppress them. createApp() installs this
// automatically; call it yourself only when wiring React manually.

export function installZoomGuard(): () => void {
  const onWheel = (e: WheelEvent) => {
    if (e.ctrlKey) e.preventDefault();
  };
  const onKey = (e: KeyboardEvent) => {
    if (e.ctrlKey && (e.key === "+" || e.key === "-" || e.key === "=" || e.key === "0")) {
      e.preventDefault();
    }
  };
  window.addEventListener("wheel", onWheel, { passive: false });
  window.addEventListener("keydown", onKey);
  return () => {
    window.removeEventListener("wheel", onWheel);
    window.removeEventListener("keydown", onKey);
  };
}
