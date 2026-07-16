import { useShell } from "./hooks";

export interface DragStripProps {
  /** Height in px; must match the Go window's CaptionHeight (default 40). */
  height?: number;
  /** Dead zone on the left; match CaptionLeftReserve (default 8). */
  leftInset?: number;
  /** Dead zone on the right for the window buttons; match CaptionRightReserve (default 150). */
  rightInset?: number;
  prefix?: string;
}

/**
 * DragStrip is the invisible band along the top of the window that
 * starts a native drag on mousedown - the frameless window's "title
 * bar surface". TitleBar renders one for you; use this directly only
 * for fully custom chrome.
 *
 * Ignores double-clicks so a quick double-tap cannot trigger the OS
 * caption double-click maximize.
 */
export function DragStrip({ height = 40, leftInset = 8, rightInset = 150, prefix }: DragStripProps) {
  const shell = useShell(prefix);
  if (!shell.available) return null;
  return (
    <div
      className="gantry-dragstrip"
      style={{ height, left: leftInset, right: rightInset }}
      onMouseDown={(e) => {
        if (e.button !== 0 || e.detail > 1) return;
        e.preventDefault();
        shell.drag();
      }}
    />
  );
}
