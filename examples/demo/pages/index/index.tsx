// The React half of the index page. This page's UI logic lives in Go
// (see index.go) - TeaView renders whatever its View() returns, and the
// layout around it is normal React.
import { TeaView } from "gantry-web/tea";
import { ExternalLink, resourceURL } from "gantry-web";

function GantryLogo() {
  return (
    <ExternalLink href="https://github.com/BlueBeard63/Gantry">
      <svg className="logo gantry" viewBox="0 0 32 32" width="96" height="96" aria-label="Gantry logo">
        <path
          d="M4 27 V10 H28 V27 M10 10 V5 H22 V10 M16 10 V17"
          stroke="currentColor"
          strokeWidth="2.5"
          strokeLinecap="round"
          strokeLinejoin="round"
          fill="none"
        />
        <circle cx="16" cy="20" r="2.6" fill="currentColor" />
      </svg>
    </ExternalLink>
  );
}

function ReactLogo() {
  return (
    <ExternalLink href="https://react.dev">
      <svg className="logo react" viewBox="-11.5 -11.5 23 23" width="96" height="96" aria-label="React logo">
        <circle r="2.05" fill="currentColor" />
        <g stroke="currentColor" strokeWidth="1" fill="none">
          <ellipse rx="11" ry="4.2" />
          <ellipse rx="11" ry="4.2" transform="rotate(60)" />
          <ellipse rx="11" ry="4.2" transform="rotate(120)" />
        </g>
      </svg>
    </ExternalLink>
  );
}

export default function Index() {
  return (
    <div className="index-page">
      <div className="logo-row">
        <GantryLogo />
        <ReactLogo />
      </div>
      <h1>Gantry + React</h1>
      <div className="card">
        <TeaView />
        <p>
          Edit <code>pages/index/index.go</code> and save - the counter lives in Go
        </p>
      </div>
      <p className="read-the-docs">
        <img src={resourceURL("logo.svg")} width={20} height={20} alt="" style={{ verticalAlign: "-4px" }} />{" "}
        <code>resources/logo.svg</code> - the same file the Go side reads via <code>gantry.Resource</code>
      </p>
      <p className="read-the-docs">
        The look lives in index.tsx, the logic in index.go - run gantry docs to learn more
      </p>
    </div>
  );
}
