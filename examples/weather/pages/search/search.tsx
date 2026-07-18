// Add Location - live geocoding search backed by the Go "weather"
// service. Typing calls weather.search (debounced); picking a result
// writes it to the shared location state (which the Go side persists)
// and returns to Settings. "Use current location" resolves to the
// default city - this build does no device GPS.
import { useEffect, useRef, useState } from "react";
import { useService, useGoState, goBack } from "gantry-web";
import { DEFAULT_LOCATION, type Location } from "../../lib/types";
import { BackIcon, CheckIcon, CloseIcon, CrosshairIcon, PinIcon, SearchIcon } from "../../lib/icons";

export default function Search() {
  const weather = useService("weather");
  const [loc, setLoc] = useGoState<Location>("location", DEFAULT_LOCATION);
  const [q, setQ] = useState("");
  const [results, setResults] = useState<Location[]>([]);
  const [loading, setLoading] = useState(false);
  const seq = useRef(0);

  // Debounce the query and ignore out-of-order responses (seq guards
  // against a slow earlier request landing after a newer one).
  useEffect(() => {
    const query = q.trim();
    if (query.length < 2) {
      setResults([]);
      setLoading(false);
      return;
    }
    const id = ++seq.current;
    setLoading(true);
    const t = setTimeout(() => {
      weather
        .call<Location[]>("search", { name: query })
        .then((r) => {
          if (id === seq.current) {
            setResults(r);
            setLoading(false);
          }
        })
        .catch(() => {
          if (id === seq.current) {
            setResults([]);
            setLoading(false);
          }
        });
    }, 250);
    return () => clearTimeout(t);
  }, [q, weather]);

  function pick(l: Location) {
    setLoc(l);
    goBack();
  }

  const isSelected = (l: Location) =>
    Math.abs(l.lat - loc.lat) < 0.01 && Math.abs(l.lon - loc.lon) < 0.01;

  return (
    <div className="wx-screen wx-search">
      <header className="wx-topbar">
        <button className="wx-iconbtn" aria-label="Back" onClick={goBack}>
          <BackIcon size={22} />
        </button>
        <span className="wx-title">Add Location</span>
      </header>

      <div className="wx-searchfield">
        <SearchIcon size={20} className="wx-search-lead" />
        <input
          className="wx-search-input"
          autoFocus
          placeholder="Search city"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        {q && (
          <button className="wx-iconbtn" aria-label="Clear" onClick={() => setQ("")}>
            <CloseIcon size={18} />
          </button>
        )}
      </div>

      <button className="wx-current" onClick={() => pick(DEFAULT_LOCATION)}>
        <CrosshairIcon size={20} />
        <span>Use current location</span>
      </button>

      <div className="wx-divider" />

      <div className="wx-matches-label">MATCHES</div>

      <ul className="wx-results">
        {results.map((r) => (
          <li key={`${r.lat},${r.lon}`}>
            <button className="wx-result" onClick={() => pick(r)}>
              <PinIcon size={20} className="wx-result-pin" />
              <span className="wx-result-text">
                <span className="wx-result-name">{r.name}</span>
                <span className="wx-result-sub">{[r.admin1, r.country].filter(Boolean).join(", ")}</span>
              </span>
              {isSelected(r) && <CheckIcon size={20} className="wx-result-check" />}
            </button>
          </li>
        ))}
        {!loading && q.trim().length >= 2 && results.length === 0 && (
          <li className="wx-empty">No matches</li>
        )}
      </ul>
    </div>
  );
}
