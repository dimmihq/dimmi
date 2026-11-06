/*
 * The button on the page that opens the messenger. The Dimmi mark turns
 * into a chevron while the panel is open; the stylesheet animates the
 * swap. A customer can give it a label, change its shape, turn it into a
 * tab on the page's edge, or hide it and open the messenger from their own
 * buttons.
 */
import type { Launcher as Settings } from "../options";
import { ChevronDownIcon, Mark } from "./icons";

export function Launcher({ launcher }: { launcher: Settings }) {
  return (
    <button
      className="launcher"
      type="button"
      data-target="launcher"
      data-action="click->toggle"
      data-shape={launcher.shape}
      data-style={launcher.style}
      aria-label="Open messages"
      aria-expanded="false"
      hidden={launcher.style === "hidden"}
    >
      <span className="icons">
        <Mark />
        <ChevronDownIcon />
      </span>
      {launcher.label ? <span className="label">{launcher.label}</span> : null}
      <span className="count" data-target="count" hidden />
    </button>
  );
}
