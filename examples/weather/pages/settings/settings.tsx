// Settings - choose the place and the unit. Both are shared Go state
// (useGoState), so a change here is live on Home immediately and the Go
// side persists it to disk. "Save" just returns to Home.
import { useGoState, navigate, goBack } from "gantry-web";
import { DEFAULT_LOCATION, type Location, type Units } from "../../lib/types";
import { BackIcon, ChevronRightIcon } from "../../lib/icons";

export default function Settings() {
  const [loc] = useGoState<Location>("location", DEFAULT_LOCATION);
  const [units, setUnits] = useGoState<Units>("units", "celsius");

  return (
    <div className="wx-screen wx-settings">
      <header className="wx-topbar">
        <button className="wx-iconbtn" aria-label="Back" onClick={goBack}>
          <BackIcon size={22} />
        </button>
        <span className="wx-title">*Settings*</span>
      </header>

      <div className="wx-fields">
        <button className="wx-field" onClick={() => navigate("/search")}>
          <span className="wx-field-label">LOCATION</span>
          <span className="wx-field-value">
            {loc.name}
            <ChevronRightIcon size={18} className="wx-field-chevron" />
          </span>
        </button>

        <button className="wx-field" onClick={() => navigate("/search")}>
          <span className="wx-field-label">COUNTRY</span>
          <span className="wx-field-value">
            {loc.country}
            <ChevronRightIcon size={18} className="wx-field-chevron" />
          </span>
        </button>

        <div className="wx-field wx-field-units">
          <span className="wx-field-label">UNITS</span>
          <div className="wx-pills">
            <button
              className={"wx-pill" + (units === "celsius" ? " is-selected" : "")}
              onClick={() => setUnits("celsius")}
            >
              °C
            </button>
            <button
              className={"wx-pill" + (units === "fahrenheit" ? " is-selected" : "")}
              onClick={() => setUnits("fahrenheit")}
            >
              °F
            </button>
          </div>
        </div>
      </div>

      <div className="wx-grow" />

      <button className="wx-save" onClick={() => navigate("/")}>
        SAVE
      </button>
    </div>
  );
}
