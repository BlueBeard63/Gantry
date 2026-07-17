// Loading states for Go calls. useCall() already exposes { loading,
// error, data }; Await is the declarative wrapper: show a developer-
// defined fallback (a spinner, <Skeleton/>, anything) while the call
// is in flight, an error card with retry when it fails, and the
// children once data is there.
//
//	const users = useCall<User[]>("api", "listUsers");
//	return (
//	  <Await call={users} fallback={<Skeleton lines={4} />}>
//	    {(list) => <ul>{list.map(...)}</ul>}
//	  </Await>
//	);

import type { CSSProperties, ReactNode } from "react";
import type { CallResult } from "./service";

export interface AwaitProps<T> {
  /** The in-flight call (from useCall, or anything CallResult-shaped). */
  call: CallResult<T>;
  /** What to show while loading: a spinner, <Skeleton/>, custom JSX.
   * Default: <Skeleton lines={3}/>. */
  fallback?: ReactNode;
  /** Custom error rendering; default is a small card with a Retry
   * button. */
  renderError?: (error: string, code: string | null, reload: () => void) => ReactNode;
  /** Renders the loaded data. */
  children: (data: T) => ReactNode;
}

export function Await<T>({ call, fallback, renderError, children }: AwaitProps<T>) {
  if (call.loading) {
    return <>{fallback ?? <Skeleton lines={3} />}</>;
  }
  if (call.error !== null) {
    if (renderError) return <>{renderError(call.error, call.code, call.reload)}</>;
    return (
      <div className="gantry-await-error">
        <span className="gantry-await-error-message">{call.error}</span>
        {call.code && <span className="gantry-await-error-code">{call.code}</span>}
        <button className="gantry-await-retry" onClick={call.reload}>
          Retry
        </button>
      </div>
    );
  }
  return <>{children(call.data as T)}</>;
}

export interface SkeletonProps {
  /** Placeholder text lines (the last one renders shorter). */
  lines?: number;
  /** Explicit size for a single block (a chart area, an avatar). */
  width?: number | string;
  height?: number | string;
  /** Round the block fully (avatars). */
  circle?: boolean;
  className?: string;
  style?: CSSProperties;
}

/**
 * Skeleton renders shimmering placeholder blocks sized like the
 * content they stand in for: <Skeleton lines={4}/> for text,
 * <Skeleton width={240} height={120}/> for a block, circle for
 * avatars.
 */
export function Skeleton({ lines, width, height, circle, className, style }: SkeletonProps) {
  if (lines === undefined && (width !== undefined || height !== undefined || circle)) {
    const s: CSSProperties = {
      width: width ?? (circle ? 40 : "100%"),
      height: height ?? (circle ? 40 : 16),
      borderRadius: circle ? "50%" : undefined,
      ...style,
    };
    return <div className={"gantry-skeleton" + (className ? " " + className : "")} style={s} />;
  }
  const n = Math.max(1, lines ?? 3);
  return (
    <div className={"gantry-skeleton-lines" + (className ? " " + className : "")} style={style}>
      {Array.from({ length: n }, (_, i) => (
        <div key={i} className="gantry-skeleton" style={{ width: i === n - 1 && n > 1 ? "60%" : "100%" }} />
      ))}
    </div>
  );
}
