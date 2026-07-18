import { useShell, useShellCaps } from "./hooks";

const EDGES = [
  { edge: "n", style: { top: 0, left: 8, right: 8, height: 5, cursor: "ns-resize" } },
  { edge: "s", style: { bottom: 0, left: 8, right: 8, height: 5, cursor: "ns-resize" } },
  { edge: "w", style: { left: 0, top: 8, bottom: 8, width: 5, cursor: "ew-resize" } },
  { edge: "e", style: { right: 0, top: 8, bottom: 8, width: 5, cursor: "ew-resize" } },
  { edge: "nw", style: { top: 0, left: 0, width: 10, height: 10, cursor: "nwse-resize" } },
  { edge: "ne", style: { top: 0, right: 0, width: 10, height: 10, cursor: "nesw-resize" } },
  { edge: "sw", style: { bottom: 0, left: 0, width: 10, height: 10, cursor: "nesw-resize" } },
  { edge: "se", style: { bottom: 0, right: 0, width: 10, height: 10, cursor: "nwse-resize" } },
] as const;

/**
 * ResizeFrame renders invisible edge strips that start a native
 * interactive resize and carry the resize cursor. A frameless window has
 * no draggable OS frame: on Linux the compositor offers no hit-test, and
 * on Windows the WebView2 child covers the client area and swallows the
 * parent's WM_NCHITTEST resize margin - so on both the frontend draws the
 * strips and calls the ResizeEdge binding on mousedown. Framed windows
 * (and the browser) render nothing.
 */
export function ResizeFrame({ prefix }: { prefix?: string }) {
  const shell = useShell(prefix);
  const caps = useShellCaps(prefix);
  const native = caps?.platform === "linux" || caps?.platform === "windows";
  if (!native || !caps?.frameless) return null;
  return (
    <>
      {EDGES.map(({ edge, style }) => (
        <div
          key={edge}
          style={{ position: "fixed", zIndex: 40, ...style }}
          onMouseDown={(e) => {
            if (e.button !== 0) return;
            e.preventDefault();
            shell.resizeEdge(edge);
          }}
        />
      ))}
    </>
  );
}
