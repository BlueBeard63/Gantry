// Inline SVG icons for the app - line-art in the spirit of the Figma
// "Lazy Weather · Components" set. Every icon draws with currentColor so
// it inherits the surrounding text colour, and takes an optional size
// (defaults to 1em, i.e. the current font size) and className.
import type { ReactElement, ReactNode, SVGProps } from "react";
import type { Trend, WxKind } from "./types";

interface IconProps extends SVGProps<SVGSVGElement> {
  size?: number | string;
}

// Base wraps the shared svg attributes so each icon is just its paths.
function Base({ size = "1em", children, ...rest }: IconProps & { children: ReactNode }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.8}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      {...rest}
    >
      {children}
    </svg>
  );
}

// --- weather ---------------------------------------------------------------

function SunIcon(p: IconProps) {
  return (
    <Base {...p}>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
    </Base>
  );
}

function CloudIcon(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M17.5 18a4.5 4.5 0 0 0 .5-8.98A6 6 0 0 0 6.2 10.2 4 4 0 0 0 7 18h10.5z" />
    </Base>
  );
}

function RainIcon(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M17.5 15a4.5 4.5 0 0 0 .5-8.98A6 6 0 0 0 6.2 7.2 4 4 0 0 0 7 15h10.5z" />
      <path d="M8 18l-1 2M12 18l-1 2M16 18l-1 2" />
    </Base>
  );
}

const WX: Record<WxKind, (p: IconProps) => ReactElement> = {
  sun: SunIcon,
  cloud: CloudIcon,
  rain: RainIcon,
};

/** WxIcon renders the weather glyph for a WMO-derived kind. */
export function WxIcon({ kind, ...rest }: IconProps & { kind: WxKind }) {
  const Cmp = WX[kind] ?? CloudIcon;
  return <Cmp {...rest} />;
}

// --- trend -----------------------------------------------------------------

/** TrendIcon shows how a value moved versus yesterday: up, down, or same. */
export function TrendIcon({ dir, ...rest }: IconProps & { dir: Trend }) {
  if (dir === "up") {
    return (
      <Base {...rest}>
        <path d="M12 19V5M6 11l6-6 6 6" />
      </Base>
    );
  }
  if (dir === "down") {
    return (
      <Base {...rest}>
        <path d="M12 5v14M6 13l6 6 6-6" />
      </Base>
    );
  }
  return (
    <Base {...rest}>
      <path d="M5 9h14M5 15h14" />
    </Base>
  );
}

// --- chrome ----------------------------------------------------------------

export function GearIcon(p: IconProps) {
  return (
    <Base {...p}>
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </Base>
  );
}

export function BackIcon(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M15 18l-6-6 6-6" />
    </Base>
  );
}

export function ChevronRightIcon(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M9 18l6-6-6-6" />
    </Base>
  );
}

export function SearchIcon(p: IconProps) {
  return (
    <Base {...p}>
      <circle cx="11" cy="11" r="7" />
      <path d="M21 21l-4.3-4.3" />
    </Base>
  );
}

export function CloseIcon(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M18 6L6 18M6 6l12 12" />
    </Base>
  );
}

export function PinIcon(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M20 10c0 6-8 12-8 12s-8-6-8-12a8 8 0 0 1 16 0z" />
      <circle cx="12" cy="10" r="3" />
    </Base>
  );
}

export function CrosshairIcon(p: IconProps) {
  return (
    <Base {...p}>
      <circle cx="12" cy="12" r="8" />
      <path d="M12 2v3M12 19v3M2 12h3M19 12h3" />
      <circle cx="12" cy="12" r="2.5" />
    </Base>
  );
}

export function CheckIcon(p: IconProps) {
  return (
    <Base {...p}>
      <path d="M20 6L9 17l-5-5" />
    </Base>
  );
}
