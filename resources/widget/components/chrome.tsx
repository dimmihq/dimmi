/*
 * The headers every view shares: a centered title for top-level tabs, and
 * a back button with the title beside it for pages opened from them. Both
 * end with the expand and close buttons, so they sit in the same place on
 * every view.
 */
import { BackIcon, CloseIcon, CollapseIcon, ExpandIcon } from "./icons";

export function Tools() {
  return (
    <span className="tools">
      <button className="icon-button" type="button" data-action="click->expand" aria-label="Expand messages">
        <ExpandIcon />
        <CollapseIcon />
      </button>
      <button className="icon-button" type="button" data-action="click->close" aria-label="Close messages">
        <CloseIcon />
      </button>
    </span>
  );
}

export function Bar({ title }: { title: string }) {
  return (
    <header className="bar">
      <span />
      <h2>{title}</h2>
      <Tools />
    </header>
  );
}

export function BackBar({ title }: { title: string }) {
  return (
    <header className="bar back-bar">
      <button className="icon-button" type="button" data-action="click->back" aria-label="Back">
        <BackIcon />
      </button>
      <h2>{title}</h2>
      <Tools />
    </header>
  );
}
