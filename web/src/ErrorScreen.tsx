// The built-in error UI: a full-screen overlay for fatal render
// crashes and dismissible banners for non-fatal errors (Go panics,
// unhandled rejections). Development shows the full story - message,
// stack, the page, and the "what led here" action trail; production
// shows a friendly minimal card. Replace it per app via
// createApp({ errors: { screen: MyScreen } }).

import type { FC } from "react";
import type { GantryCrumb, GantryErrorInfo } from "./errors";

export interface ErrorScreenProps {
  error: GantryErrorInfo;
  /** Resolved app mode; null while unknown (treated as development). */
  mode: "development" | "production" | null;
  /** "fatal" = render crash overlay, "notice" = dismissible banner. */
  variant: "fatal" | "notice";
  onDismiss: () => void;
}

function ago(iso: string, now: number): string {
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return "";
  const s = Math.max(0, Math.round((now - t) / 1000));
  if (s < 1) return "now";
  if (s < 60) return `${s}s ago`;
  return `${Math.floor(s / 60)}m${s % 60 ? ` ${s % 60}s` : ""} ago`;
}

function Trail({ trail }: { trail: GantryCrumb[] }) {
  const now = Date.now();
  return (
    <div className="gantry-error-trail">
      <div className="gantry-error-heading">What led here</div>
      {trail.map((c, i) => (
        <div key={i} className={"gantry-error-crumb" + (c.ok ? "" : " gantry-error-crumb-failed")}>
          <span className="gantry-error-crumb-time">{ago(c.time, now)}</span>
          <span className="gantry-error-crumb-type">{c.type}</span>
          <span className="gantry-error-crumb-detail">{c.detail}</span>
        </div>
      ))}
    </div>
  );
}

function Detail({ error }: { error: GantryErrorInfo }) {
  return (
    <>
      <div className="gantry-error-meta">
        <span className="gantry-error-kind">{error.kind}</span>
        {error.code && <span className="gantry-error-code">{error.code}</span>}
        {error.page && <span className="gantry-error-page">on {error.page}</span>}
        {error.source && <span className="gantry-error-source">{error.source}</span>}
        <span className="gantry-error-origin">{error.origin === "go" ? "Go" : "JS"}</span>
      </div>
      {error.stack && <pre className="gantry-error-stack">{error.stack}</pre>}
      {error.componentStack && (
        <>
          <div className="gantry-error-heading">Component stack</div>
          <pre className="gantry-error-stack">{error.componentStack}</pre>
        </>
      )}
      {error.trail && error.trail.length > 0 && <Trail trail={error.trail} />}
    </>
  );
}

export const ErrorScreen: FC<ErrorScreenProps> = ({ error, mode, variant, onDismiss }) => {
  const dev = mode !== "production";
  if (variant === "fatal") {
    return (
      <div className="gantry-error-overlay">
        <div className="gantry-error-card">
          <div className="gantry-error-title">{dev ? "The page crashed" : "Something went wrong"}</div>
          {dev ? <div className="gantry-error-message">{error.message}</div> : <div className="gantry-error-message">The app hit an unexpected error. Reloading usually fixes it.</div>}
          {dev && <Detail error={error} />}
          <div className="gantry-error-actions">
            <button className="gantry-error-button" onClick={() => location.reload()}>
              Reload
            </button>
          </div>
        </div>
      </div>
    );
  }
  return (
    <div className="gantry-error-banner">
      <div className="gantry-error-banner-head">
        <span className="gantry-error-banner-title">
          {error.origin === "go" ? "Go error" : "Error"}: {error.message}
        </span>
        <button className="gantry-error-dismiss" onClick={onDismiss} aria-label="Dismiss">
          ×
        </button>
      </div>
      {dev && <Detail error={error} />}
    </div>
  );
};
