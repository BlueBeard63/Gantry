// App-wide options (optional file): customizes createApp without
// touching the synthesized entry. Here: title on the left and
// back/forward buttons in the top bar's left slot. The Go window's
// CaptionLeftReserve is raised to match in main.go - the two halves of
// the clickable-zone contract.
import { goBack, goForward, type CreateAppOptions } from "gantry-web";
import { ArrowLeft, ArrowRight } from "lucide-react";

export default {
  title: "Demo",
  titleBar: {
    titleAlign: "left",
    leftReserve: 100,
    left: (
      <>
        <button className="gantry-tbbtn" onClick={goBack} aria-label="Back">
          <ArrowLeft size={15} />
        </button>
        <button className="gantry-tbbtn" onClick={goForward} aria-label="Forward">
          <ArrowRight size={15} />
        </button>
      </>
    ),
  },
} satisfies CreateAppOptions;
