// Home - the "lazy weather" screen. The header (location + settings) is
// React chrome; the body is whatever the Go "weather" service returns
// for the current location and unit, wrapped in <Await> so it shows a
// skeleton while loading and a retry card on failure.
import { Await, Skeleton, useCall, useGoState, navigate } from "gantry-web";
import { DEFAULT_LOCATION, type Forecast, type Location, type Units } from "../../lib/types";
import { GearIcon, TrendIcon, WxIcon } from "../../lib/icons";

export default function Home() {
  const [loc] = useGoState<Location>("location", DEFAULT_LOCATION);
  const [units] = useGoState<Units>("units", "celsius");
  const fc = useCall<Forecast>("weather", "forecast", { lat: loc.lat, lon: loc.lon, units });

  return (
    <div className="wx-screen wx-home">
      <header className="wx-topbar">
        <span className="wx-location">*{loc.name}*</span>
        <button className="wx-iconbtn" aria-label="Settings" onClick={() => navigate("/settings")}>
          <GearIcon size={22} />
        </button>
      </header>

      <Await call={fc} fallback={<HomeSkeleton />}>
        {(f) => (
          <div className="wx-body">
            <div className="wx-grow" />

            <section className="wx-statement">
              <p>Today's weather is</p>
              <p className="wx-dim">{f.compare}</p>
              <p>as yesterday</p>
              {f.detail && <p className="wx-statement-detail">{f.detail}</p>}
            </section>

            <div className="wx-grow" />

            <ul className="wx-forecast">
              {f.rows.map((r) => (
                <li className="wx-row" key={r.label}>
                  <span className="wx-row-label">{r.label}</span>
                  <span className="wx-row-temp">
                    {r.temp}°{f.unitSign}
                  </span>
                  <WxIcon kind={r.icon} size={18} className="wx-glyph" />
                  <span className="wx-row-gap" />
                  <span className="wx-row-delta">
                    {Math.abs(r.delta)}°{f.unitSign}
                  </span>
                  <TrendIcon dir={r.trend} size={16} className="wx-glyph" />
                </li>
              ))}
            </ul>
          </div>
        )}
      </Await>
    </div>
  );
}

// HomeSkeleton mirrors the body layout so the screen doesn't jump when
// the forecast lands: statement placeholder mid-screen, four rows bottom.
function HomeSkeleton() {
  return (
    <div className="wx-body">
      <div className="wx-grow" />
      <section className="wx-statement">
        <Skeleton width={190} height={20} />
        <Skeleton width={160} height={20} />
        <Skeleton width={120} height={20} />
      </section>
      <div className="wx-grow" />
      <ul className="wx-forecast">
        {[0, 1, 2, 3].map((i) => (
          <li key={i} className="wx-row-skeleton">
            <Skeleton height={16} />
          </li>
        ))}
      </ul>
    </div>
  );
}
