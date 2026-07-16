import { useEffect, useState, type ReactNode } from "react";
import { Minus, Square, Copy, X } from "lucide-react";
import { useShell, useShellCaps } from "./hooks";
import { DragStrip } from "./DragStrip";

export interface TitleBarProps {
  /** Centered title content; pointer-transparent so it stays draggable. */
  title?: ReactNode;
  /** Extra content on the left (menu buttons, an icon). */
  left?: ReactNode;
  /** Extra content on the right, before the window buttons. */
  right?: ReactNode;
  /** Bar height in px; MUST match the Go window's CaptionHeight (default 40). */
  height?: number;
  /**
   * Width reserved for the button cluster; MUST match the Go window's
   * CaptionRightReserve (default 150). The Go side keeps this zone
   * clickable instead of draggable.
   */
  rightReserve?: number;
  /** Override the Caps()-driven visibility per button. */
  showMinimize?: boolean;
  showMaximize?: boolean;
  showClose?: boolean;
  /** Tooltip for the close button, e.g. "Keeps running in the tray". */
  closeHint?: string;
  /** Replace the default close action (shell.close). */
  onClose?: () => void;
  prefix?: string;
  className?: string;
}

/**
 * TitleBar draws the custom window chrome: a drag strip, an optional
 * centered title, and the window buttons. Which buttons appear comes
 * from the Go window's Caps() - configure buttons once, in Go. In a
 * plain browser the window buttons hide themselves.
 */
export function TitleBar(props: TitleBarProps) {
  const {
    title,
    left,
    right,
    height = 40,
    rightReserve = 150,
    closeHint,
    onClose,
    prefix,
    className,
  } = props;
  const shell = useShell(prefix);
  const caps = useShellCaps(prefix);
  const [maximized, setMaximized] = useState(false);

  const showMin = props.showMinimize ?? caps?.minimize ?? false;
  const showMax = props.showMaximize ?? caps?.maximize ?? false;
  const showClose = props.showClose ?? caps?.close ?? false;

  useEffect(() => {
    if (showMax) void shell.isMaximized().then(setMaximized);
  }, [shell, showMax]);

  return (
    <div className={"gantry-titlebar " + (className ?? "")} style={{ height }}>
      <DragStrip height={height} rightInset={rightReserve} prefix={prefix} />
      {left != null && <div className="gantry-titlebar-left">{left}</div>}
      {title != null && (
        <div className="gantry-titlebar-title" style={{ lineHeight: height + "px" }}>
          {title}
        </div>
      )}
      <div className="gantry-titlebar-buttons" style={{ minWidth: right != null ? undefined : 0 }}>
        {right}
        {showMin && (
          <button
            type="button"
            className="gantry-winbtn"
            aria-label="Minimize"
            onClick={() => shell.minimize()}
          >
            <Minus size={16} />
          </button>
        )}
        {showMax && (
          <button
            type="button"
            className="gantry-winbtn"
            aria-label={maximized ? "Restore" : "Maximize"}
            onClick={() => {
              if (maximized) shell.restore();
              else shell.maximize();
              setMaximized(!maximized);
            }}
          >
            {maximized ? <Copy size={14} /> : <Square size={14} />}
          </button>
        )}
        {showClose && (
          <button
            type="button"
            className="gantry-winbtn gantry-winbtn-close"
            aria-label="Close"
            title={closeHint}
            onClick={() => (onClose ? onClose() : shell.close())}
          >
            <X size={16} />
          </button>
        )}
      </div>
    </div>
  );
}
